package example

import (
	"github.com/flipped-aurora/gin-vue-admin/server/global"
)

// ExaFile File structure for breakpoint continuation
type ExaFile struct {
	global.GVA_MODEL
	FileName     string
	FileMd5      string
	FilePath     string
	ExaFileChunk []ExaFileChunk
	ChunkTotal   int
	IsFinish     bool
}

// ExaFileChunk File chunk structure for breakpoint continuation
type ExaFileChunk struct {
	global.GVA_MODEL
	ExaFileID       uint
	FileChunkNumber int
	FileChunkPath   string
}
