package rrs

import (
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"github.com/facebookgo/ensure"
	"github.com/teh-cmc/seq"
)

// -----------------------------------------------------------------------------

func TestRRServer_LockedIDMap_DumpAndLoad(t *testing.T) {
	f, err := ioutil.TempFile("", "TestLockedIDMap_DumpAndLoad")
	ensure.Nil(t, err)
	defer os.Remove(f.Name())

	m1 := &lockedIDMap{
		RWMutex: &sync.RWMutex{},
		ids: map[string]*lockedID{
			"A": &lockedID{id: 42},
			"B": &lockedID{id: 66},
			"C": &lockedID{id: seq.ID(1e6)},
		},
	}
	m2 := &lockedIDMap{RWMutex: &sync.RWMutex{}, ids: make(map[string]*lockedID)}

	ensure.Nil(t, m1.Load(f)) // empty file should be ignored
	ensure.Nil(t, m2.Load(f)) // empty file should be ignored

	ensure.Nil(t, m1.Dump(f))
	ensure.Nil(t, m2.Load(f))

	for name, lid := range m1.ids {
		ensure.DeepEqual(t, lid.id, m2.ids[name].id)
	}

	m2.ids["B"].id = seq.ID(666)

	ensure.Nil(t, m2.Dump(f))
	ensure.Nil(t, m1.Load(f))

	ensure.DeepEqual(t, m1.ids["B"].id, seq.ID(666))
	for name, lid := range m2.ids {
		ensure.DeepEqual(t, lid.id, m1.ids[name].id)
	}

	_, err = f.WriteString("dumping some random garbage")
	ensure.Nil(t, err)
	ensure.Nil(t, m1.Dump(f))
	ensure.Nil(t, m1.Dump(f))
	ensure.Nil(t, m1.Dump(f))
	ensure.Nil(t, m1.Dump(f))
	ensure.Nil(t, m1.Load(f))

	for name, lid := range m2.ids {
		ensure.DeepEqual(t, lid.id, m1.ids[name].id)
	}
}
