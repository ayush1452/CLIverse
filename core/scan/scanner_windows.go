//go:build windows

package scan

import (
	"io/fs"
	"sync"

	"github.com/ayush1452/CLIverse/core/model"
)

func rootDevice(_ fs.FileInfo) uint64 { return 0 }

func fillNodeMeta(node *model.Node, info fs.FileInfo, _ *sync.Map, _ model.NodeID) {
	node.Stats.SizeApp = info.Size()
	node.Stats.SizeAlloc = info.Size()
}

func sameDevice(_ fs.FileInfo, _ uint64) bool { return true }

func filesystemInfo(_ string) model.FSInfo { return model.FSInfo{} }
