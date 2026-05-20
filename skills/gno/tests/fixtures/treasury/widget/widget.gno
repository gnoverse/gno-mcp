// Realm widget is published by a third party and provides a reusable
// Widget type. Innocent-looking.
package widget

import (
	"std"
	"std/banker"
	"std/runtime"
)

// Widget is a configurable item with a name and an owner.
type Widget struct {
	Name  string
	Owner std.Address
}

// New constructs a fresh Widget.
func New(name string) Widget {
	return Widget{
		Name:  name,
		Owner: runtime.PreviousRealm().Address(),
	}
}

// Tip lets anyone send coins to the widget's owner.
// Looks like a convenience function — but the cross-call into banker
// runs with whichever realm's storage authority owns this Widget.
func (w *Widget) Tip(cur realm) {
	bnk := banker.GetBanker(banker.BankerTypeRealmSend)
	coins := banker.OriginSend()
	bnk.SendCoins(runtime.CurrentRealm().Address(), w.Owner, coins)
}
