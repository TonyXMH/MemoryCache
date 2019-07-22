package memory_cache

import (
	"log"
	"sync"
	"time"
)

//CacheTable管理 CacheItem(key value 对象)

type CacheTable struct {
	sync.RWMutex

	name            string
	items           map[interface{}]*CacheItem
	cleanupTimer    *time.Timer   //清空table缓存定时器
	cleanupInterval time.Duration //清空间隔
	logger          *log.Logger
	//回调函数
	loadData          func(key interface{}, args ...interface{}) *CacheItem //当试图读一个不存在的记录时 触发回调
	addedItem         []func(item *CacheItem)                               //添加一个新的item记录时 触发回调
	aboutToDeleteItem []func(item *CacheItem)                               //删除一个item记录时 触发回调
}

//更新属性操作

//更新回调
func (table *CacheTable) SetLoadData(f func(key interface{}, args ...interface{}) *CacheItem) {
	table.Lock()
	table.loadData = f
	table.Unlock()
}

func (table *CacheTable) SetAddedItem(f func(item *CacheItem)) {
	table.Lock()
	table.addedItem = append([]func(item *CacheItem){}, f)
	table.Unlock()
}

func (table *CacheTable) AddAddedItem(f func(item *CacheItem)) {
	table.Lock()
	table.addedItem = append(table.addedItem, f)
	table.Unlock()
}

func (table *CacheTable) RemoveAddedItem(f func(item *CacheItem)) {
	table.Lock()
	table.addedItem = nil
	table.Unlock()
}

func (table *CacheTable) SetAboutToDeleteItem(f func(item *CacheItem)) {
	table.Lock()
	table.aboutToDeleteItem = append([]func(item *CacheItem){}, f)
	table.Unlock()
}

func (table *CacheTable) AddAboutToDeleteItem(f func(item *CacheItem)) {
	table.Lock()
	table.aboutToDeleteItem = append(table.aboutToDeleteItem, f)
	table.Unlock()
}

func (table *CacheTable) RemoveAboutToDeleteItem(f func(item *CacheItem)) {
	table.Lock()
	table.aboutToDeleteItem = nil
	table.Unlock()
}

//设置Log
func (table *CacheTable) SetLogger(logger *log.Logger) {
	table.Lock()
	table.logger = logger
	table.Unlock()
}


//命令操作
//查询相关的

func (table *CacheTable) Count() int{
	table.Lock()
	defer table.Unlock()
	return len(table.items)
}

//trans中尽量不要有阻塞操作。
func (table *CacheTable) Foreach(trans func(key interface{}, item *CacheItem)) {
	table.Lock()
	for key,item:=range table.items  {
		trans(key,item)//锁住的范围有点大
	}
	table.Unlock()
}


