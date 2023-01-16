package nft

import (
	"math"
	"sync"

	"github.com/H-BF/sgroups/cmd/to-nft/internal/nft/cases"
	model "github.com/H-BF/sgroups/internal/models/sgroups"
	"github.com/c-robinson/iplib"
	nftLib "github.com/google/nftables"
	nftLibUtil "github.com/google/nftables/binaryutil"
	"github.com/pkg/errors"
)

type nfTablesTx struct {
	*nftLib.Conn
	commitOnce sync.Once
}

func nfTx() (*nfTablesTx, error) {
	c, e := nftLib.New(nftLib.AsLasting())
	if e != nil {
		return nil, errors.WithMessage(e, "open nft tx")
	}
	return &nfTablesTx{Conn: c}, nil
}

func (tx *nfTablesTx) applyIPSets(tbl *nftLib.Table, agg cases.IPsBySG, ipV ipVersion) error {
	const api = "ntf/apply-IP-sets"

	for _, x := range agg {
		if x.IPs.Len() == 0 {
			continue
		}
		ipSet := &nftLib.Set{
			Table: tbl,
			Name:  nameUtils{}.nameOfAddrSet(ipV, x.SG.Name),
		}
		switch ipV {
		case ipV4:
			ipSet.KeyType = nftLib.TypeIPAddr
		case ipV6:
			ipSet.KeyType = nftLib.TypeIP6Addr
		default:
			panic("wrong ipV is passed")
		}
		var elements []nftLib.SetElement
		for _, ip := range x.IPs {
			ipAsInt := iplib.IPToBigint(ip)
			b := ipAsInt.Bytes()
			reverseBytes(b)
			elements = append(elements,
				nftLib.SetElement{
					Key: b,
				})
		}
		if err := tx.AddSet(ipSet, elements); err != nil {
			return errors.WithMessagef(err, "%s: add set", api)
		}
	}
	return nil
}

func (tx *nfTablesTx) applyPortSets(tbl *nftLib.Table, agg cases.SgToSgs) error {
	const api = "ntf/apply-port-sets"

	var (
		names nameUtils
		err   error
		be    = nftLibUtil.BigEndian
	)
	for k, items := range agg {
		for _, item := range *items {
			pranges := []model.PortRanges{item.PortsFrom, item.PortsTo}
			for i := range pranges {
				portSet := &nftLib.Set{
					Table:    tbl,
					Name:     names.nameOfPortSet(k.Transport, k.SgFrom, item.SgTo, i > 0),
					KeyType:  nftLib.TypeInetService,
					Interval: true,
				}
				var elemnts []nftLib.SetElement
				pranges[i].Iterate(func(r model.PortRange) bool {
					a, b := r.Bounds()
					b = b.AsExcluded()
					aVal, _ := a.GetValue()
					bVal, _ := b.GetValue()
					if aVal > math.MaxUint16 || bVal > math.MaxUint16 {
						err = ErrPortRange
						return false //error
					}
					elemnts = append(elemnts,
						nftLib.SetElement{
							Key: be.PutUint16(uint16(aVal)), KeyEnd: be.PutUint16(uint16(bVal)),
						})
					return true
				})
				if err != nil {
					return errors.WithMessage(err, api)
				}
				if err = tx.AddSet(portSet, elemnts); err != nil {
					return errors.WithMessagef(err, "%s: add set", api)
				}
			}
		}
	}
	return nil
}

func (tx *nfTablesTx) commit() error {
	const api = "ntf/flush"

	c := tx.Conn
	var err error
	var passed bool
	tx.commitOnce.Do(func() {
		err = c.Flush()
		_ = c.CloseLasting()
		passed = true
	})
	if passed {
		return errors.WithMessage(err, api)
	}
	return errors.Errorf("%s: closed", api)
}

func (tx *nfTablesTx) abort() {
	c := tx.Conn
	tx.commitOnce.Do(func() {
		_ = c.CloseLasting()
	})
}

func reverseBytes(p []byte) {
	for i, j := 0, len(p)-1; i < j && j >= 0; i, j = i+1, j-1 {
		p[i], p[j] = p[j], p[i]
	}
}