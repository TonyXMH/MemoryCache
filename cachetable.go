package memory_cache

import (
	"log"
	"sort"
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

func (table *CacheTable) RemoveAddedItem() {
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

func (table *CacheTable) RemoveAboutToDeleteItem() {
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

func (table *CacheTable) Count() int {
	table.Lock()
	defer table.Unlock()
	return len(table.items)
}

//trans中尽量不要有阻塞操作。
func (table *CacheTable) Foreach(trans func(key interface{}, item *CacheItem)) {
	table.Lock()
	for key, item := range table.items {
		trans(key, item) //锁住的范围有点大
	}
	table.Unlock()
}

//向table中添加item对象，lifeSpan 为0 表示永久有效 会有覆盖添加的情况发生
func (table *CacheTable) Add(key, data interface{}, lifeSpan time.Duration) *CacheItem {
	item := NewCacheItem(key, data, lifeSpan)
	table.addInternal(item)
	return item
}

func (table *CacheTable) addInternal(item *CacheItem) {
	table.Lock()
	table.log("Adding item with key ", item.key, " and life span of ", item.lifeSpan, " to table ", table.name)
	table.items[item.key] = item
	//利用临时变量缩短临界区
	expDur := table.cleanupInterval
	addedItem := table.addedItem
	table.Unlock()
	//个人认为这里callback（item）并不安全，callback就算修改item，那也只是顺序修改，这里创建的对象并没有被其他goroutine访问到
	if addedItem != nil {
		for _, callback := range addedItem {
			callback(item)
		}
	}

	if item.lifeSpan > 0 && (expDur == 0 || item.lifeSpan < expDur) {
		//当对象有超时信息，需要过期检查
		table.expirationCheck()
	}
}

//对整个table中所有item记录整体做过期检查
func (table *CacheTable) expirationCheck() {
	table.Lock()
	//并发Add的时候，g1,g2,g3，g2和g3其实就是在更新检查，所以直接关闭定时器，接下来会更新检测时间
	if table.cleanupTimer != nil {
		table.cleanupTimer.Stop()
	}
	if table.cleanupInterval > 0 {
		table.log("Expiration check triggered after ", table.cleanupInterval, " for table ", table.name)
	} else {
		table.log("Expiration check has done for table", table.name)
	}
	//过期检查是对于整个items进行的
	now := time.Now()
	smallestDuration := 0 * time.Second
	for key, item := range table.items {
		//获取每个item的过期信息,缩减临界区
		item.RLock()
		lifeSpan := item.lifeSpan
		accessedOn := item.accessedOn
		item.RUnlock()

		if lifeSpan == 0 { //该item永久有效
			continue
		}
		if now.Sub(accessedOn) > lifeSpan { //item过期 删除item
			table.deleteInternal(key)
		} else { //item未过期，更新查找table中距离过期最近的时间
			if smallestDuration == 0 || lifeSpan-now.Sub(accessedOn) < smallestDuration {
				smallestDuration = lifeSpan - now.Sub(accessedOn)
			}
		}

		table.cleanupInterval = smallestDuration //cleanup定时器将在最近过期的时间触发回调，删除过期item
		if smallestDuration > 0 {                //设置超时回调，回调永远在本函数结束后被触发，所以不会出现死锁的情况
			table.cleanupTimer = time.AfterFunc(smallestDuration, func() { //这样确保了每次临近超时都会有goroutine处理
				go table.expirationCheck() //有必要go出去吗
			})
		}
	}
	table.Unlock()
}

//deleteInternal的实现有点想条件变量，内部件解锁后加锁，以便达到减少临界区的目的
func (table *CacheTable) deleteInternal(key interface{}) (*CacheItem, error) {
	item, ok := table.items[key]
	if !ok {
		return nil, ErrNotFound
	}
	aboutToDeleteItem := table.aboutToDeleteItem
	table.Unlock() //先解锁减少临界区域
	//aboutToDeleteItem回调的触发时间先于delete
	for _, callback := range aboutToDeleteItem {
		callback(item) //这里的操作可能会有数据一致性的问题吧？
	}

	item.RLock()
	if item.aboutToExpire != nil {
		for _, callback := range item.aboutToExpire {
			callback(key) //这里的操作可能会有数据一致性的问题吧？
		}
	}
	item.RUnlock()
	table.Lock()
	table.log("Deleting item with key ", key, "created on ", item.createdOn, " and hit ", item.accessedCount, " from table", table.name)
	delete(table.items, key)
	return item, nil
}

func (table *CacheTable) Delete(key interface{}) (*CacheItem, error) {
	table.Lock()
	defer table.Unlock()
	return table.deleteInternal(key)
}

func (table *CacheTable) Exists(key interface{}) bool {
	table.RLock()
	_, ok := table.items[key]
	table.RUnlock()
	return ok
}

func (table *CacheTable) NotFoundAdd(key, data interface{}, lifeSpan time.Duration) bool {
	if table.Exists(key) {
		return false
	}
	item := NewCacheItem(key, data, lifeSpan)
	table.addInternal(item)
	return true
}

func (table *CacheTable) Value(key interface{}, args ...interface{}) (*CacheItem,error) {
	table.RLock()//减少临界区域
	item, ok := table.items[key]
	loadData:=table.loadData
	table.RUnlock()

	if ok{//被访问后更新访问信息
		item.KeepAlive()
		return item,nil
	}
	//当试图访问一个不存在的key时
	if loadData!=nil{
		item=loadData(key,args)//先触发访问不存在key时的回调
		if item !=nil{
			table.Add(key,item.data,item.lifeSpan)
			return item,nil
		}
		return nil,ErrNotFoundOrLoadable
	}

	//没有设置loadData就直接返回没有找到
	return nil,ErrNotFound
}

//删除该table中的所有item
func (table *CacheTable) Flush() {
	table.Lock()
	table.log("Flushing table ",table.name)
	table.items=make(map[interface{}]*CacheItem)//丢弃所有的item指向新的内存
	table.cleanupInterval = 0
	if table.cleanupTimer!=nil{
		table.cleanupTimer.Stop()
	}
	table.Unlock()
}

type CacheItemPair struct {
	Key interface{}
	AccessedCount int64
}

type CacheItemPairList []CacheItemPair

func (p CacheItemPairList)Swap(i,j int)  {
	p[i],p[j] = p[j],p[i]
}
func (p CacheItemPairList)Len() int {
	return len(p)
}
func (p CacheItemPairList)Less(i,j int)bool  {//i>j 就不换从大到小排序
	return p[i].AccessedCount > p[j].AccessedCount
}
//获取前count个 访问次数较多的item
func (table *CacheTable) MostAccessed(count int)[]*CacheItem {
	itemPairList:=make(CacheItemPairList,len(table.items))
	table.RLock()//用一个范围较大的锁，保证了改阶段中不会被其他线程阻塞
	i:=0
	for k,v:=range table.items{
		itemPairList[i] = CacheItemPair{k,v.accessedCount}
		i++
	}

	sort.Sort(itemPairList)

	returnItems:=[]*CacheItem{}
	for i,v:=range itemPairList{
		if i < count{
			//table.RLock()
			item,ok:= table.items[v.Key]
			if ok{
				returnItems = append(returnItems,item)
			}
			//table.RUnlock()
		}
	}
	table.RUnlock()
	return returnItems
}

func (table *CacheTable) log(v ...interface{}) {
	if table.logger != nil {
		table.logger.Println(v...)
	}
}

