package memory_cache

import "sync"

var (
	cache = make(map[string]*CacheTable)//可以认为是数据库的database中存有多张表
	mutex sync.RWMutex
)

func Cache(name string)*CacheTable  {
	mutex.RLock()
	cacheTable,ok:=cache[name]
	mutex.RUnlock()

	if !ok{//没有已有的表则新建一个
		//double check
		mutex.Lock()
		cacheTable,ok=cache[name]
		if !ok{
			cacheTable=&CacheTable{
				name:              name,
				items:             make(map[interface{}]*CacheItem),
			}
			cache[name] = cacheTable
		}
		mutex.Unlock()
	}
	return cacheTable
}