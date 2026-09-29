package server

import (
	"os"
	"sync"
	"time"
)

// fileCache 缓存「从磁盘读出并解析好的」索引文件，文件修改时间或大小变化才重新解析。
// 图标索引（hdicons.json / nasicon.json）单个可达数 MB，后台站点列表会为每个站点查一次
// 「自动匹配到哪个图标」，每次都读盘 + 反序列化开销很大。
type fileCache[T any] struct {
	mu      sync.Mutex
	modTime time.Time
	size    int64
	value   *T
}

// load 返回 path 的解析结果；文件不存在时返回 nil。read 只在文件变化时调用。
func (f *fileCache[T]) load(path string, read func(string) *T) *T {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.value != nil && info.ModTime().Equal(f.modTime) && info.Size() == f.size {
		return f.value
	}
	f.value = read(path)
	f.modTime, f.size = info.ModTime(), info.Size()
	return f.value
}
