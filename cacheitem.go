package memory_cache

import (
	"sync"
	"time"
)

//cache key value 记录对象

type CacheItem struct {
	sync.RWMutex //共享锁来控制访问时间与次数并发修改

	key  interface{}
	data interface{}

	lifeSpan      time.Duration //生命周期(过期时间)
	createdOn     time.Time     //被创建时间
	accessedOn    time.Time     //被访问时间
	accessedCount int64         //被访问的次数

	aboutToExpire []func(key interface{}) //记录被移除后的回调函数组
}

func NewCacheItem(key, data interface{}, lifeSpan time.Duration) *CacheItem {
	t := time.Now()
	return &CacheItem{
		RWMutex:       sync.RWMutex{},
		key:           key,
		data:          data,
		lifeSpan:      lifeSpan,
		createdOn:     t,
		accessedOn:    t,
		accessedCount: 0,
		aboutToExpire: nil,
	}
}

//以下都是查询操作:

//item对象的key和data是不进行修改的
func (item *CacheItem) Key() interface{} {
	return item.key
}

func (item *CacheItem) Data() interface{} {
	return item.data
}

func (item *CacheItem) LifeSpan() time.Duration {
	return time.Duration(item.lifeSpan)
}

func (item *CacheItem) CreatedOn() time.Time {
	return item.createdOn
}

func (item *CacheItem) AccessedOn() time.Time {
	item.RLock()
	defer item.RUnlock()
	return item.accessedOn
}

func (item *CacheItem) AccessedCount() int64 {
	item.RLock()
	defer item.RUnlock()
	return item.accessedCount
}

//以下都是更新操作
//保活的操作,每当Value读取后，都会重置删除定时。
func (item *CacheItem) KeepAlive() {
	item.Lock()
	item.accessedOn = time.Now()
	item.accessedCount++
	item.Unlock()
}

//更新回调操作
//Set是先清空之前的回调，再将新增的回调添加进去
func (item *CacheItem) SetAboutToExpireCallBack(f func(interface{})) {
	item.Lock()
	item.aboutToExpire = append([]func(key interface{}){},f)
	item.Unlock()

}
func (item *CacheItem) AddAboutToExpireCallBack(f func(key interface{})) {
	item.Lock()
	item.aboutToExpire=append(item.aboutToExpire,f)//频繁添加内存开辟的问题，但是这里应该不会用那么多。
	item.Unlock()
}
func (item *CacheItem) RemoveAboutToExpireCallBack() {
	item.Lock()
	item.aboutToExpire=nil//由gc回收内存
	item.Unlock()
}