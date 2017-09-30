package main

import (
	"errors"
	"time"
)

//基础定时器为每小时，匹配hour触发动作
func Cron(h, m int) (<-chan time.Time, error) {
	//因为只监控凌晨的时段，所以限定过滤器为0-5时
	if h < 0 || h > 23 {
		return nil, errors.New("hour must between 0 and 23")
	}
	if m < 0 || m > 59 {
		return nil, errors.New("miniute mutst between 0 and 59")
	}
	var tick = time.Tick(1 * time.Minute)
	var cron = make(chan time.Time)
	go func() {
		for now := range tick {
			//send time to cron, when h, m matched
			hour, min, _ := now.Clock()
			if hour == h && m == min {
				cron <- now
			}
		}
	}()
	return cron, nil
}
