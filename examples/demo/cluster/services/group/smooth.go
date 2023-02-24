package group

// EMA is exponential moving average
func EMA(smoothingFactor float64, x, sPrev float64) float64 {
	return smoothingFactor*x + (1-smoothingFactor)*sPrev
}

// SMA is simple moving average
func SMA(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := float64(0)
	for _, s := range data {
		sum += s
	}
	return sum / float64(len(data))
}

// RW is random walk
func RW() float64 {
	return 0
}

// TODO: 1. 实现数据平滑算法 2. 实现插值机制 3. 实现预测机制 4. 实现延迟补偿机制 5. 实现仿真系统 6. 实现数据协议 7. 实现预测错误和解机制
