package memory_cache

import (
	"bytes"
	"log"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var (
	k = "TestKey"
	v = "TestValue"
)

func TestCache(t *testing.T) {
	table := Cache("TestCache")
	table.Add(k+"_1", v, 0)
	table.Add(k+"_2", v, time.Second)

	item, err := table.Value(k + "_1")
	if err != nil || item == nil || item.Data().(string) != v {
		t.Error("Error retrieving non expiring data from cache", err)
	}
	item, err = table.Value(k + "_2")
	if err != nil || item == nil || item.Data().(string) != v {
		t.Error("Error retrieving non expiring data from cache ", err)
	}

	if item.AccessedCount() != 1 {
		t.Error("Error get wrong access count")
	}
	if item.LifeSpan() != time.Second {
		t.Error("Error get wrong life span")
	}

	if item.AccessedOn().Unix() == 0 {
		t.Error("Error get wrong accessed time")
	}
	if item.CreatedOn().Unix() == 0 {
		t.Error("Error get wrong created time")
	}
}

func TestCacheExpire(t *testing.T) {
	table := Cache("TestCacheExpire")
	table.Add(k+"_1", v+"_1", 100*time.Millisecond)
	table.Add(k+"_2", v+"_2", 125*time.Millisecond)
	time.Sleep(75 * time.Millisecond)
	_, err := table.Value(k + "_1") //重置删除定时
	if err != nil {
		t.Error("Error retrieving value from cache ", err)
	}
	time.Sleep(75 * time.Millisecond)
	_, err = table.Value(k + "_1") //之前被重置过
	if err != nil {
		t.Error("Error retrieving value from cache ", err)
	}
	_, err = table.Value(k + "_2")
	if err == nil {
		t.Error("Found key which should have been expired by now", err)
	}
}

func TestExists(t *testing.T) {
	table := Cache("TestExists")
	table.Add(k, v, 0)
	if !table.Exists(k) {
		t.Error("Error verifying existing data in cache")
	}
	table.Add(k+"_1", v+"_1", 10*time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	if !table.Exists(k + "_1") {
		t.Error("Error verifying existing data in cache")
	}
	time.Sleep(7 * time.Millisecond)
	if table.Exists(k + "_1") { //过期不应该存在
		t.Error("Error expire data should not be in cache ")
	}
}

func TestNotFoundAdd(t *testing.T) {
	table := Cache("TestNotFoundAdd")
	if !table.NotFoundAdd(k, v, 0) {
		t.Error("Error verifying NotFoundAdd, data not in cache")
	}

	if table.NotFoundAdd(k, v, 0) {
		t.Error("Error key has already added in cache,but we can't find it")
	}
}

func TestNotFoundAddConcurrency(t *testing.T) {
	table := Cache("TestNotFoundAddConcurrency")
	var finished sync.WaitGroup
	var added int32
	var idle int32
	fn := func(id int) {
		defer finished.Done()
		for i := 0; i < 100; i++ {
			if table.NotFoundAdd(i, i+id, 0) {
				atomic.AddInt32(&added, 1)
			} else {
				atomic.AddInt32(&idle, 1)
			}
		}
	}

	finished.Add(10)
	for i := 0; i < 10; i++ {
		go fn(i * 100)
	}
	finished.Wait()
	t.Log(added, idle)
	//table.Foreach(func(key interface{}, item *CacheItem) {
	//	v,_:=item.Data().(int)
	//	k,_:=key.(int)
	//	t.Logf("k%+v,v%+v",k,v)
	//})
}

func TestCacheKeepAlive(t *testing.T) {
	table := Cache("TestCacheKeepAlive")
	item := table.Add(k, v, 100*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	item.KeepAlive()
	time.Sleep(75 * time.Millisecond)
	if !table.Exists(k) {
		t.Errorf("k%+v should not expire after keep alive", k)
	}

	time.Sleep(75 * time.Millisecond)
	if table.Exists(k) {
		t.Errorf("k%+v should be expired not alive", k)
	}
}

func TestDelete(t *testing.T) {
	table := Cache("TestDelete")
	table.Add(k, v, 0)
	item, err := table.Value(k)
	if err != nil || item == nil || item.Data().(string) != v {
		t.Error("Error retrieving data from cache", err)
	}
	table.Delete(k)
	item, err = table.Value(k)
	if err == nil || item != nil {
		t.Error("Error delete data")
	}

}

func TestFlush(t *testing.T) {
	table := Cache("TestFlush")
	table.Add(k, v, 10*time.Second)
	table.Flush()

	item, err := table.Value(k)
	if err == nil || item != nil {
		t.Error("Error flushing table")
	}

	if table.Count() != 0 {
		t.Error("Error verifying count of flushed table")
	}

}

func TestCount(t *testing.T) {
	table := Cache("TestCount")
	count := 100000

	for i := 0; i < count; i++ {
		key := k + strconv.Itoa(i)
		table.Add(key, v, 10*time.Second)
	}

	for i := 0; i < count; i++ {
		key := k + strconv.Itoa(i)
		item, err := table.Value(key)
		if err != nil || item == nil || item.Data().(string) != v {
			t.Error("Error retrieving data")
		}
	}
	if table.Count() != count {
		t.Error("Error retrieving data")
	}
}

func TestDataLoader(t *testing.T) {
	table := Cache("TestDataLoader")
	table.SetLoadData(func(key interface{}, args ...interface{}) *CacheItem {
		var item *CacheItem
		if key.(string) != "nil" {
			val := k + key.(string)
			i := NewCacheItem(key, val, 500*time.Millisecond)
			item = i
		}
		return item
	})

	_, err := table.Value("nil")
	if err == nil || table.Exists("nil") {
		t.Error("Error validating data loader for nil values")
	}

	for i := 0; i < 10; i++ {
		key := k + strconv.Itoa(i)
		val := k + key
		item, err := table.Value(key)
		if err != nil || item == nil || item.Data().(string) != val {
			t.Error("Error validating data loader")
		}
	}
}

func TestAccessCount(t *testing.T) {
	count := 100
	table := Cache("TestAccessCount")

	for i := 0; i < count; i++ {
		table.Add(i, v, 10*time.Second)
	}

	for i := 0; i < count; i++ {
		for j := 0; j < i; j++ {
			table.Value(i)
		}
	}
	ma := table.MostAccessed(count)
	for i, item := range ma {
		if item.Key() != count-1-i {
			t.Error("Most accessed items seem to be sorted incorrectly")
		}
	}

	ma=table.MostAccessed(count -1)
	if len(ma)!=count -1{
		t.Error("MostAccessed returns incorrect amount of items")
	}
}

func TestCallbacks(t *testing.T)  {
	var m sync.Mutex
	addedKey:=""
	removedKey:=""
	calledAddedItem:=false
	calledRemovedItem:=false
	expired:=false
	calledExpired:=false

	table:=Cache("TestCallbacks")
	table.SetAddedItem(func(item *CacheItem) {
		m.Lock()
		addedKey=item.Key().(string)
		m.Unlock()
	})
	table.SetAddedItem(func(item *CacheItem) {
		m.Lock()
		calledAddedItem=true
		m.Unlock()
	})
	table.SetAboutToDeleteItem(func(item *CacheItem) {
		m.Lock()
		removedKey = item.Key().(string)
		m.Unlock()
	})
	table.SetAboutToDeleteItem(func(item *CacheItem) {
		m.Lock()
		calledRemovedItem = true
		m.Unlock()
	})

	item:=table.Add(k,v,500*time.Millisecond)
	item.SetAboutToExpireCallBack(func(key interface{}) {
		m.Lock()
		expired = true
		m.Unlock()
	})
	item.SetAboutToExpireCallBack(func(key interface{}) {
		m.Lock()
		calledExpired = true
		m.Unlock()
	})

	time.Sleep(250*time.Millisecond)
	m.Lock()
	if addedKey == k && !calledAddedItem{
		t.Error("AddedItem callback not working")
	}
	m.Unlock()

	time.Sleep(500*time.Millisecond)
	m.Lock()
	if removedKey == k && !calledRemovedItem{
		t.Error("AboutToDeleteItem callback not working: "+k+"_"+removedKey)
	}
	if expired && !calledExpired{
		t.Error("AboutToExpire callback not working")
	}
	m.Unlock()
}

func TestCallbackQueue(t*testing.T)  {
	var m sync.Mutex
	addedKey:=""
	addedKeyCallback2:=""
	secondCallbackResult:="second"
	removedKey:=""
	removedKeyCallback:=""
	expired:=false
	calledExpired:=false

	table:=Cache("TestCallbackQueue")
	table.SetAddedItem(func(item *CacheItem) {
		m.Lock()
		addedKey=item.Key().(string)
		m.Unlock()
	})
	table.SetAddedItem(func(item *CacheItem) {
		m.Lock()
		addedKeyCallback2=secondCallbackResult
		m.Unlock()
	})
	table.SetAboutToDeleteItem(func(item *CacheItem) {
		m.Lock()
		removedKey = item.Key().(string)
		m.Unlock()
	})
	table.SetAboutToDeleteItem(func(item *CacheItem) {
		m.Lock()
		removedKeyCallback = secondCallbackResult
		m.Unlock()
	})

	item:=table.Add(k,v,500*time.Millisecond)
	item.SetAboutToExpireCallBack(func(key interface{}) {
		m.Lock()
		expired = true
		m.Unlock()
	})
	item.SetAboutToExpireCallBack(func(key interface{}) {
		m.Lock()
		calledExpired = true
		m.Unlock()
	})

	time.Sleep(250*time.Millisecond)
	m.Lock()
	if addedKey == k && addedKeyCallback2!=secondCallbackResult{
		t.Error("AddedItem callback not working")
	}
	m.Unlock()

	time.Sleep(500*time.Millisecond)
	m.Lock()
	if removedKey == k && removedKeyCallback!=secondCallbackResult{
		t.Error("AboutToDeleteItem callback not working: "+k+"_"+removedKey)
	}
	m.Unlock()

	table.RemoveAddedItem()
	table.RemoveAboutToDeleteItem()
	secondItemKey:="itemKey02"
	expired = false
	item=table.Add(secondItemKey,v,500*time.Millisecond)
	item.SetAboutToExpireCallBack(func(key interface{}) {
		m.Lock()
		expired = true
		m.Unlock()
	})

	item.RemoveAboutToExpireCallBack()
	time.Sleep(250*time.Millisecond)
	m.Lock()
	if addedKey == secondItemKey{
		t.Error("AddedItemCallbacks were not removed")
	}
	m.Unlock()

	time.Sleep(500*time.Millisecond)
	m.Lock()
	if removedKey == secondItemKey{
		t.Error("AboutToDeleteItem not removed")
	}
	if !expired && !calledExpired{
		t.Error("AboutToExpire callback not working")
	}
	m.Unlock()
}

func TestLogger(t *testing.T){
	out:=new(bytes.Buffer)
	l:=log.New(out,"memory-cache",log.Ldate|log.Ltime)
	table:=Cache("TestLogger")
	table.SetLogger(l)
	table.Add(k,v,0)
	time.Sleep(100*time.Millisecond)

	if out.Len() == 0{
		t.Error("Logger is empty")
	}
	//fmt.Print(out)
}
