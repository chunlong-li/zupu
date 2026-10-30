package handler

import (
	"fmt"
	"sync"
	"time"
)

// 更新进度阶段
const (
	stageDownloading = "downloading" // 下载中（percent 0-100）
	stageExtracting  = "extracting"  // 解压中
	stageRestarting  = "restarting"  // 重启中
	stageDone        = "done"        // 更新完毕
	stageError       = "error"       // 出错
)

// UpdateProgress 单次更新任务的进度快照
type UpdateProgress struct {
	Stage   string `json:"stage"`
	Percent int    `json:"percent"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

// progressStore 更新任务进度存储（UpdateHandler 每次请求新建，故用包级全局 + 锁）
var progressStore = struct {
	sync.Mutex
	jobs map[string]*UpdateProgress
}{jobs: map[string]*UpdateProgress{}}

// newUpdateTask 注册一个新的更新任务，返回任务 id
func newUpdateTask() string {
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	progressStore.Lock()
	progressStore.jobs[id] = &UpdateProgress{Stage: stageDownloading, Percent: 0, Message: "准备下载"}
	progressStore.Unlock()
	return id
}

func setUpdateProgress(id, stage string, percent int, message string) {
	progressStore.Lock()
	if j, ok := progressStore.jobs[id]; ok {
		j.Stage = stage
		j.Percent = percent
		j.Message = message
	}
	progressStore.Unlock()
}

func setUpdateError(id, msg string) {
	progressStore.Lock()
	if j, ok := progressStore.jobs[id]; ok {
		j.Stage = stageError
		j.Error = msg
	}
	progressStore.Unlock()
}

func getUpdateProgress(id string) *UpdateProgress {
	progressStore.Lock()
	defer progressStore.Unlock()
	if j, ok := progressStore.jobs[id]; ok {
		cp := *j
		return &cp
	}
	return nil
}
