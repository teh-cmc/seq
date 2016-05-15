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

	ensure.Nil(t, m1.Dump(f))
	ensure.Nil(t, m2.Load(f))

	for name, lid := range m1.ids {
		ensure.DeepEqual(t, lid.id, m2.ids[name].id)
	}
}
