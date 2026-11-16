// Package api HTTP 处理层（Gin handler）。
package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"omnimux-inspiration/internal/errs"
	"omnimux-inspiration/internal/media"
	"omnimux-inspiration/internal/model"
	"omnimux-inspiration/internal/service"
)

type CoverReader interface {
	Read(context.Context, string) (*media.CoverImage, error)
}

type MediaStreamer interface {
	Open(context.Context, string, string, string) (*media.MediaStream, error)
}

type Handler struct {
	Covers         CoverReader
	MediaStreamer  MediaStreamer
	Svc            *service.Service
	Media          *media.Store
	MediaURLPrefix string
}

func New(svc *service.Service, mediaStore *media.Store, mediaURLPrefix string) *Handler {
	return &Handler{Svc: svc, Media: mediaStore, MediaURLPrefix: mediaURLPrefix}
}

// ---------- 统一包络 ----------

func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data, "request_id": c.GetString("request_id")})
}

func fail(c *gin.Context, err error) {
	e := errs.FromErr(err)
	c.AbortWithStatusJSON(e.Status, gin.H{
		"success": false, "code": e.Code, "message": e.Msg, "request_id": c.GetString("request_id"),
	})
}

// ---------- inspirations ----------

func (h *Handler) List(c *gin.Context) {
	q := &service.ListQuery{
		Type: c.Query("type"),
		Tag:  c.Query("tag"),
		Q:    c.Query("q"),
		Sort: c.DefaultQuery("sort", "hot"),
	}
	if v := c.Query("tags"); v != "" {
		for _, t := range strings.Split(v, ",") {
			if t = strings.TrimSpace(t); t != "" {
				q.Tags = append(q.Tags, t)
			}
		}
	}
	if v := c.Query("is_favorite"); v != "" {
		b := v == "true" || v == "1"
		q.IsFavorite = &b
	}
	if err := bindFilters(c, q); err != nil {
		fail(c, err)
		return
	}
	q.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	q.Size, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))

	res, err := h.Svc.List(c.Request.Context(), q)
	if err != nil {
		fail(c, err)
		return
	}
	h.decorateMedia(res.Items)
	ok(c, res)
}

func (h *Handler) Get(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		fail(c, errs.ErrInvalidParam)
		return
	}
	rec, err := h.Svc.Get(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	h.decorateOne(rec)
	ok(c, rec)
}

func (h *Handler) Create(c *gin.Context) {
	var req service.CreateInspirationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, errs.ErrInvalidParam)
		return
	}
	returnExisting := c.Query("return_existing") == "true"
	rec, existed, err := h.Svc.Create(c.Request.Context(), &req, returnExisting)
	if err != nil {
		fail(c, err)
		return
	}
	h.decorateOne(rec)
	status := http.StatusOK
	if !existed {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{"success": true, "data": rec, "existed": existed, "request_id": c.GetString("request_id")})
}

func (h *Handler) Bulk(c *gin.Context) {
	var req service.BulkReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, errs.ErrInvalidParam)
		return
	}
	res, err := h.Svc.BulkUpsertTx(c.Request.Context(), &req)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, res)
}

func (h *Handler) Update(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		fail(c, errs.ErrInvalidParam)
		return
	}
	var req service.UpdateInspirationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, errs.ErrInvalidParam)
		return
	}
	rec, err := h.Svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		fail(c, err)
		return
	}
	h.decorateOne(rec)
	ok(c, rec)
}

func (h *Handler) Delete(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		fail(c, errs.ErrInvalidParam)
		return
	}
	if err := h.Svc.Delete(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"deleted": true})
}

// ---------- tags ----------

func (h *Handler) ListTags(c *gin.Context) {
	tags, err := h.Svc.ListTags(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, tags)
}

func (h *Handler) CreateTag(c *gin.Context) {
	var body struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		fail(c, errs.ErrInvalidParam)
		return
	}
	tag, err := h.Svc.CreateTag(c.Request.Context(), body.Name)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": tag, "request_id": c.GetString("request_id")})
}

// UploadMedia 接收 multipart file 或 JSON {url, kind}，存盘/同步 R2，返回短 key 与绝对 path。
func (h *Handler) UploadMedia(c *gin.Context) {
	if h.Media == nil {
		fail(c, errs.Wrap("MEDIA_DISABLED", http.StatusServiceUnavailable, "media storage disabled", nil))
		return
	}
	kind := c.DefaultQuery("kind", "cover")
	if kind != "cover" && kind != "media" {
		fail(c, errs.ErrInvalidParam)
		return
	}

	contentType := c.GetHeader("Content-Type")
	// 分支 1：JSON URL 导入模式
	if strings.HasPrefix(contentType, "application/json") {
		var body struct {
			URL  string `json:"url"`
			Kind string `json:"kind"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.URL) == "" {
			fail(c, errs.ErrInvalidParam)
			return
		}
		if body.Kind != "" {
			kind = body.Kind
		}
		res, err := h.Media.SaveURL(c.Request.Context(), kind, body.URL)
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusCreated, gin.H{"success": true, "data": res, "request_id": c.GetString("request_id")})
		return
	}

	// 分支 2：Multipart 文件上传
	fileHeader, err := c.FormFile("file")
	if err != nil {
		fail(c, errs.ErrInvalidParam)
		return
	}
	f, err := fileHeader.Open()
	if err != nil {
		fail(c, errs.ErrInvalidParam)
		return
	}
	defer f.Close()

	raw, err := io.ReadAll(f)
	if err != nil {
		fail(c, errs.Wrap("INTERNAL", 500, "read upload file", err))
		return
	}
	res, err := h.Media.SaveBytes(c.Request.Context(), kind, raw)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": res, "request_id": c.GetString("request_id")})
}

func (h *Handler) DeleteTag(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		fail(c, errs.ErrInvalidParam)
		return
	}
	if err := h.Svc.DeleteTag(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"deleted": true})
}

// ---------- helpers ----------

func parseID(c *gin.Context) (uint64, error) {
	return strconv.ParseUint(c.Param("id"), 10, 64)
}

// decorateMedia 把 media_keys/cover_key 装饰成可访问 URL 路径。
func (h *Handler) decorateMedia(items []model.Inspiration) {
	for i := range items {
		h.decorateOne(&items[i])
	}
}

func (h *Handler) decorateOne(item *model.Inspiration) {
	prefix := h.MediaURLPrefix + "/"
	if strings.HasPrefix(item.CoverKey, media.CoverRefPrefix) {
		item.CoverKey = prefix + "inspiration-covers/" + strconv.FormatUint(item.ID, 10)
	} else if item.CoverKey != "" && !strings.HasPrefix(item.CoverKey, "http") {
		item.CoverKey = prefix + item.CoverKey
	}
	if len(item.MediaKeys) > 0 {
		var keys []string
		if err := jsonUnmarshal(item.MediaKeys, &keys); err == nil {
			for j, k := range keys {
				if !strings.HasPrefix(k, "http") {
					keys[j] = prefix + k
				}
			}
			item.MediaKeys = mustJSON(keys)
		}
	}
}

// StreamMedia proxies a r2/publications/... object from R2. It is only used
// when the sidecar has no local copy, preserving the existing local-first path.
func (h *Handler) StreamMedia(c *gin.Context, key string) {
	if h.MediaStreamer == nil {
		c.Status(http.StatusNotFound)
		return
	}
	stream, err := h.MediaStreamer.Open(c.Request.Context(), key, c.GetHeader("Range"), c.GetHeader("If-Range"))
	if err != nil {
		e := errs.FromErr(err)
		if e.Status == http.StatusNotFound {
			c.Status(http.StatusNotFound)
			return
		}
		c.AbortWithStatusJSON(e.Status, gin.H{
			"success": false, "code": e.Code, "message": e.Msg, "request_id": c.GetString("request_id"),
		})
		return
	}
	defer stream.Body.Close()
	for name, value := range map[string]string{
		"Content-Type": stream.ContentType, "Content-Length": stream.ContentLength,
		"Content-Range": stream.ContentRange, "Accept-Ranges": stream.AcceptRanges,
		"ETag": stream.ETag, "Last-Modified": stream.LastModified, "Cache-Control": stream.CacheControl,
	} {
		if value != "" {
			c.Header(name, value)
		}
	}
	c.Status(stream.Status)
	_, _ = io.Copy(c.Writer, stream.Body)
}

func (h *Handler) Cover(c *gin.Context, rawID string) {
	id, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || id == 0 || strconv.FormatUint(id, 10) != rawID {
		fail(c, errs.ErrInvalidParam)
		return
	}
	ref, err := h.Svc.CoverReference(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	if h.Covers == nil {
		fail(c, media.ErrCoverUnavailable)
		return
	}
	cover, err := h.Covers.Read(c.Request.Context(), ref)
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Content-Type", cover.ContentType)
	c.Header("Cache-Control", "no-store")
	if cover.ETag != "" {
		c.Header("ETag", cover.ETag)
	}
	http.ServeContent(c.Writer, c.Request, "cover", cover.LastModified, bytes.NewReader(cover.Data))
}

var errBind = errors.New("bind")
