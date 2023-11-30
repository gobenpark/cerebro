package strategy

const (
	// CacheKeyPrefix prefix of cache key
	CacheKeyPrefix = "cerebro"
	// CacheKeyTick tick cache key
	CacheKeyTick = "tick"
	// CacheKeyCandle candle cache key
	CacheKeyCandle = "candle"
	// CacheKeyOrder order cache key
	CacheKeyOrder = "order"
)

func CandleKey(code string, compress string) string {
	return CacheKeyPrefix + ":" + CacheKeyCandle + ":" + code + ":" + compress
}
