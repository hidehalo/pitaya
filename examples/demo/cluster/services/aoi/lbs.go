package aoi

import (
	"github.com/tidwall/rtree"
)

// // create a 2D RTree
// var tr rtree.RTree

// // insert a point
// tr.Insert([2]float64{-112.0078, 33.4373}, [2]float64{-112.0078, 33.4373}, "PHX")

// // insert a rect
// tr.Insert([2]float64{10, 10}, [2]float64{20, 20}, "rect")

// // search
// tr.Search([2]float64{-112.1, 33.4}, [2]float64{-112.0, 33.5},
// 	func(min, max [2]float64, data interface{}) bool {
// 		println(data.(string)) // prints "PHX"
// 		return true
// 	})

// // delete
// tr.Delete([2]float64{-112.0078, 33.4373}, [2]float64{-112.0078, 33.4373}, "PHX")

// // search
// tr.Search([2]float64{-112.1, 33.4}, [2]float64{-112.0, 33.5},
// 	func(min, max [2]float64, data interface{}) bool {
// 		println(data.(string)) // prints "PHX"
// 		return true
// 	})

type LBS struct {
	rTree *rtree.RTree
}

type Location struct {
	X float64
	Y float64
}

func NewLBS() *LBS {
	lbs := &LBS{
		rTree: new(rtree.RTree),
	}
	return lbs
}

func (lbs *LBS) AddLocation(loc Location, data interface{}) {
	locRect := [2]float64{loc.X, loc.Y}
	lbs.rTree.Insert(locRect, locRect, data)
}

func (lbs *LBS) ReplaceLocation(old, new Location, oldData, newData interface{}) {
	oldRect := [2]float64{old.X, old.Y}
	newRect := [2]float64{new.X, new.Y}
	lbs.rTree.Replace(oldRect, oldRect, oldData, newRect, newRect, newData)
}

func (lbs *LBS) Nearby(loc Location, radius float64) []interface{} {
	dataSlice := make([]interface{}, 0)
	iter := func(min [2]float64, max [2]float64, data interface{}) bool {
		dataSlice = append(dataSlice, data)
		return true
	}
	lbs.rTree.Search([2]float64{loc.X - radius, loc.Y - radius}, [2]float64{loc.X + radius, loc.Y + radius}, iter)
	return dataSlice
}
