//go:build !windows

package scan

import (
	"io/fs"
	"sync"
	"syscall"

	"github.com/ayush1452/CLIverse/core/model"
)

func rootDevice(info fs.FileInfo) uint64 {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(stat.Dev)
	}
	return 0
}

func fillNodeMeta(node *model.Node, info fs.FileInfo, seenInodes *sync.Map, id model.NodeID) {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		node.Meta.UID = stat.Uid
		node.Meta.GID = stat.Gid
		node.Meta.Ident = model.FileIdentity{
			Dev:   uint64(stat.Dev),
			Inode: stat.Ino,
		}
		node.Stats.SizeApp = info.Size()
		node.Stats.SizeAlloc = stat.Blocks * 512

		if stat.Nlink > 1 && node.Kind == model.KindFile {
			if _, seen := seenInodes.LoadOrStore(node.Meta.Ident, id); seen {
				node.Flags.IsHardlink = true
				node.Stats.SizeAlloc = 0
				node.Stats.SizeApp = 0
			}
		}
	} else {
		node.Stats.SizeApp = info.Size()
		node.Stats.SizeAlloc = info.Size()
	}
}

func sameDevice(info fs.FileInfo, rootDev uint64) bool {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return uint64(stat.Dev) == rootDev
	}
	return true
}

func filesystemInfo(path string) model.FSInfo {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return model.FSInfo{}
	}
	blockSize := uint64(stat.Bsize)
	return model.FSInfo{
		MountPoint: path,
		TotalBytes: int64(stat.Blocks * blockSize),
		FreeBytes:  int64(stat.Bavail * blockSize),
	}
}
