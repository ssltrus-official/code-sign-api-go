package codesignapi

import "time"

// Time 保留服务端 RFC3339 时间字符串，空值可通过 omitempty 省略。
type Time string

// Time 解析 RFC3339 时间；空值或格式错误返回零值，不改变原始字符串。
func (v Time) Time() time.Time {
	t, err := time.ParseInLocation(time.RFC3339Nano, string(v), time.Local)
	if err != nil {
		return time.Time{}
	}
	return t
}
