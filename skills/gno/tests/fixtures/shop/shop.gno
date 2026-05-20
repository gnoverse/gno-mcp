// Realm shop sells items for ugnot.
package shop

import (
	"std"
	"std/banker"
	"std/runtime"
	"strconv"

	"gno.land/p/demo/avl"
)

const (
	price    = 100
	itemSKU  = "widget"
)

var (
	admin     = std.Address("g1qz8e0fz3y0pl9y4dq9d7c5dwnyu6qf04hs7z0a")
	inventory = avl.NewTree()
)

// Buy purchases one item for `price` ugnot.
func Buy(cur realm) {
	if !runtime.PreviousRealm().IsUser() {
		panic("EOA only")
	}
	coins := banker.OriginSend()
	if coins.AmountOf("ugnot") != price {
		panic("incorrect amount")
	}
	current, _ := inventory.Get(itemSKU)
	n, _ := current.(int)
	if n == 0 {
		panic("out of stock")
	}
	inventory.Set(itemSKU, n-1)
}

// Stock replenishes inventory. Admin only.
func Stock(cur realm, qty int) {
	if runtime.PreviousRealm().Address() != admin {
		panic("admin only")
	}
	current, _ := inventory.Get(itemSKU)
	n, _ := current.(int)
	inventory.Set(itemSKU, n+qty)
}

// SetCallback registers a hook called after each Buy.
// Caller-supplied: must trust the callback's source realm.
var afterBuy func(realm)

func SetCallback(cur realm, cb func(realm)) {
	if runtime.PreviousRealm().Address() != admin {
		panic("admin only")
	}
	afterBuy = cb
}

func Render(_ string) string {
	current, _ := inventory.Get(itemSKU)
	n, _ := current.(int)
	return "# Shop\n\n**" + itemSKU + "**: " + strconv.Itoa(n) + " in stock, " + strconv.Itoa(price) + " ugnot each.\n"
}
