package timeutil

import (
	"strings"
	"time"
)

var (
	BeijingZone *time.Location
	EasternZone *time.Location
	UTCZone     *time.Location
)

const (
	DateTimeLayout = "2006-01-02 15:04:05"
	DateLayout     = "2006-01-02"
)

func init() {
	var err error
	BeijingZone, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		BeijingZone = time.FixedZone("CST", 8*3600)
	}

	EasternZone, err = time.LoadLocation("America/New_York")
	if err != nil {
		EasternZone = time.FixedZone("EST", -5*3600)
	}

	UTCZone = time.UTC
}

// ParseBjToEt 将北京时间字符串 ("2006-01-02 15:04:05") 转换为美东时间
func ParseBjToEt(bjTimeStr string) *time.Time {
	bjTimeStr = strings.TrimSpace(bjTimeStr)
	if bjTimeStr == "" {
		return nil
	}
	t, err := time.ParseInLocation(DateTimeLayout, bjTimeStr, BeijingZone)
	if err != nil {
		return nil
	}
	et := t.In(EasternZone)
	return &et
}

// ParseBjToEtDate 提取美东时间的日期 (yyyy-MM-dd)
func ParseBjToEtDate(bjTimeStr string) string {
	et := ParseBjToEt(bjTimeStr)
	if et == nil {
		return ""
	}
	return et.Format(DateLayout)
}

// ConvertBjToEt 将北京时间转换为美东时间
func ConvertBjToEt(bjTime *time.Time) *time.Time {
	if bjTime == nil {
		return nil
	}
	et := bjTime.In(EasternZone)
	return &et
}

// ConvertBjToUtc 将北京时间转换为 UTC 时间
func ConvertBjToUtc(bjTime *time.Time) *time.Time {
	if bjTime == nil {
		return nil
	}
	utc := bjTime.In(UTCZone)
	return &utc
}

// ParseBjToUtc 将北京时间字符串转换为 UTC 时间
func ParseBjToUtc(bjTimeStr string) *time.Time {
	bjTimeStr = strings.TrimSpace(bjTimeStr)
	if bjTimeStr == "" {
		return nil
	}
	t, err := time.ParseInLocation(DateTimeLayout, bjTimeStr, BeijingZone)
	if err != nil {
		return nil
	}
	utc := t.In(UTCZone)
	return &utc
}

func GetTodayEt() string {
	return time.Now().In(EasternZone).Format(DateLayout)
}

func GetTodayUtc() string {
	return time.Now().In(UTCZone).Format(DateLayout)
}

func GetTodayCst() string {
	return time.Now().In(BeijingZone).Format(DateLayout)
}

// GetMaxToday 获取北京时间、美东时间与 UTC 时间中的最大日期 (对应 Java TimeUtils)
func GetMaxToday() time.Time {
	todayBj := time.Now().In(BeijingZone)
	todayEt := time.Now().In(EasternZone)
	todayUtc := time.Now().In(UTCZone)

	maxToday := todayBj
	if todayEt.After(maxToday) {
		maxToday = todayEt
	}
	if todayUtc.After(maxToday) {
		maxToday = todayUtc
	}
	return maxToday
}
