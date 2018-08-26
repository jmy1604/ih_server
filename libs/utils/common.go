package utils

func Int64From2Int32(high int32, low int32) int64 {
	return ((int64(high) << 32) & 0x7fffffff00000000) + int64(low)
}

func TwoInt32FromInt64(value int64) (int32, int32) {
	high := int32((value >> 32) & 0xffffffff)
	low := int32(value & 0xffffffff)
	return high, low
}
