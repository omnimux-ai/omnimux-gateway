package jina

// JinaReaderBaseURL 是 Jina Reader（URL → LLM 友好文本）的独立 host，
// 与 embeddings/rerank 所在的 api.jina.ai 不同。
const JinaReaderBaseURL = "https://r.jina.ai"

var ModelList = []string{
	"jina-clip-v1",
	"jina-reranker-v2-base-multilingual",
	"jina-reranker-m0",
	"jina-reader-v1",
}

var ChannelName = "jina"
