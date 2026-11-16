package media

import (
	"context"
	"encoding/binary"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"omnimux-inspiration/internal/config"
)

var testJPEG = []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x01, 0x00, 0x48}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return New(&config.Config{MediaDir: t.TempDir(), MediaURLPrefix: "/media", MaxUploadMB: 1})
}

func TestSaveURLRejectsRedirectToPrivateAddressBeforeAccess(t *testing.T) {
	var privateHits atomic.Int32
	privateServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		privateHits.Add(1)
		_, _ = w.Write(testJPEG)
	}))
	defer privateServer.Close()

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, privateServer.URL, http.StatusFound)
	}))
	defer redirectServer.Close()

	store := newTestStore(t)
	baseDialer := &net.Dialer{}
	store.httpClient.Transport = &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if strings.HasPrefix(address, "93.184.216.34:") {
				return baseDialer.DialContext(ctx, network, redirectServer.Listener.Addr().String())
			}
			return baseDialer.DialContext(ctx, network, address)
		},
	}
	publicURL := "http://93.184.216.34:" + redirectServer.URL[strings.LastIndex(redirectServer.URL, ":")+1:] + "/cover.jpg"
	if _, err := store.SaveURL(t.Context(), "cover", publicURL); err == nil {
		t.Fatal("redirect to a private address succeeded")
	}
	if got := privateHits.Load(); got != 0 {
		t.Fatalf("private redirect target was accessed %d time(s)", got)
	}
}

func TestSaveURLAcceptsControlledPublicTLSFixture(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverName := ""
		if r.TLS != nil {
			serverName = r.TLS.ServerName
		}
		if serverName != "example.com" {
			t.Errorf("TLS server name = %q, want example.com", serverName)
		}
		_, _ = w.Write(testJPEG)
	}))
	defer server.Close()

	client := newHTTPClient(nil)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T", client.Transport)
	}
	if transport.DialTLSContext != nil || transport.TLSClientConfig != nil {
		t.Fatal("production transport replaced Go's default TLS/SNI path")
	}
	testTransport := transport.Clone()
	serverTransport := server.Client().Transport.(*http.Transport)
	testTransport.TLSClientConfig = serverTransport.TLSClientConfig.Clone()
	testTransport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	client.Transport = testTransport

	store := newTestStore(t)
	store.httpClient = client
	_, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatalf("split TLS fixture address: %v", err)
	}
	stored, err := store.SaveURL(t.Context(), "cover", "https://example.com:"+port+"/cover.jpg")
	if err != nil {
		t.Fatalf("save controlled public URL: %v", err)
	}
	if stored.ContentType != "image/jpeg" || stored.Size != int64(len(testJPEG)) {
		t.Fatalf("stored media = %+v", stored)
	}
}

func TestSafeDialerRejectsDNSResolutionToPrivateAddress(t *testing.T) {
	var targetHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetHits.Add(1)
		_, _ = w.Write(testJPEG)
	}))
	defer target.Close()

	resolver, dnsQueries := newLoopbackResolver(t)
	store := newTestStore(t)
	store.httpClient = newHTTPClient(resolver)
	_, port, err := net.SplitHostPort(target.Listener.Addr().String())
	if err != nil {
		t.Fatalf("split target address: %v", err)
	}
	if _, err := store.SaveURL(t.Context(), "cover", "http://rebind.example:"+port+"/cover.jpg"); err == nil {
		t.Fatal("DNS resolution to a private address succeeded")
	}
	if dnsQueries.Load() == 0 {
		t.Fatal("controlled DNS resolver was not queried")
	}
	if got := targetHits.Load(); got != 0 {
		t.Fatalf("private DNS target was accessed %d time(s)", got)
	}
}

func TestHTTPClientDisablesEnvironmentProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	client := newHTTPClient(nil)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T", client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("media HTTP transport uses a proxy function")
	}
}

func TestBlockedIPRejectsNonPublicRanges(t *testing.T) {
	tests := []struct {
		address string
		blocked bool
	}{
		{address: "8.8.8.8", blocked: false},
		{address: "0.0.0.1", blocked: true},
		{address: "0.255.255.255", blocked: true},
		{address: "1.0.0.0", blocked: false},
		{address: "127.0.0.1", blocked: true},
		{address: "::ffff:127.0.0.1", blocked: true},
		{address: "169.254.10.20", blocked: true},
		{address: "::", blocked: true},
		{address: "100.63.255.255", blocked: false},
		{address: "100.64.0.1", blocked: true},
		{address: "100.127.255.255", blocked: true},
		{address: "100.128.0.0", blocked: false},
		{address: "::ffff:100.127.255.254", blocked: true},
		{address: "192.0.0.1", blocked: true},
		{address: "192.0.1.255", blocked: false},
		{address: "192.0.2.1", blocked: true},
		{address: "192.88.99.1", blocked: true},
		{address: "198.18.0.1", blocked: true},
		{address: "198.51.100.1", blocked: true},
		{address: "203.0.113.1", blocked: true},
		{address: "239.255.255.255", blocked: true},
		{address: "240.0.0.0", blocked: true},
		{address: "255.255.255.255", blocked: true},
		{address: "2001:4860:4860::8888", blocked: false},
		{address: "2001::1", blocked: true},
		{address: "2001:db8::1", blocked: true},
		{address: "2002::1", blocked: true},
		{address: "3fff::1", blocked: true},
		{address: "4000::1", blocked: true},
	}
	for _, test := range tests {
		t.Run(test.address, func(t *testing.T) {
			if got := blockedIP(net.ParseIP(test.address)); got != test.blocked {
				t.Fatalf("blockedIP(%s) = %v, want %v", test.address, got, test.blocked)
			}
		})
	}
}

func newLoopbackResolver(t *testing.T) (*net.Resolver, *atomic.Int32) {
	t.Helper()
	dns, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for controlled DNS: %v", err)
	}
	t.Cleanup(func() { _ = dns.Close() })
	queries := &atomic.Int32{}
	go func() {
		buf := make([]byte, 1500)
		for {
			n, addr, err := dns.ReadFrom(buf)
			if err != nil {
				return
			}
			response, ok := loopbackDNSResponse(buf[:n])
			if !ok {
				continue
			}
			queries.Add(1)
			_, _ = dns.WriteTo(response, addr)
		}
	}()
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "udp", dns.LocalAddr().String())
		},
	}
	return resolver, queries
}

func loopbackDNSResponse(query []byte) ([]byte, bool) {
	if len(query) < 17 {
		return nil, false
	}
	questionEnd := 12
	for {
		if questionEnd >= len(query) {
			return nil, false
		}
		labelLen := int(query[questionEnd])
		questionEnd++
		if labelLen == 0 {
			break
		}
		if labelLen&0xc0 != 0 || questionEnd+labelLen > len(query) {
			return nil, false
		}
		questionEnd += labelLen
	}
	if questionEnd+4 > len(query) {
		return nil, false
	}
	questionEnd += 4
	queryType := binary.BigEndian.Uint16(query[questionEnd-4 : questionEnd-2])
	var answer []byte
	switch queryType {
	case 1:
		answer = net.IPv4(127, 0, 0, 1).To4()
	case 28:
		answer = net.ParseIP("::ffff:127.0.0.1").To16()
	default:
		return nil, false
	}

	response := append([]byte(nil), query[:questionEnd]...)
	binary.BigEndian.PutUint16(response[2:4], 0x8180)
	binary.BigEndian.PutUint16(response[4:6], 1)
	binary.BigEndian.PutUint16(response[6:8], 1)
	binary.BigEndian.PutUint16(response[8:10], 0)
	binary.BigEndian.PutUint16(response[10:12], 0)
	response = append(response, 0xc0, 0x0c)
	response = binary.BigEndian.AppendUint16(response, queryType)
	response = binary.BigEndian.AppendUint16(response, 1)
	response = binary.BigEndian.AppendUint32(response, 0)
	response = binary.BigEndian.AppendUint16(response, uint16(len(answer)))
	response = append(response, answer...)
	return response, true
}
