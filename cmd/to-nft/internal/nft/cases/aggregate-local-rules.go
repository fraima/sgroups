package cases

import (
	"context"
	"net"

	conv "github.com/H-BF/sgroups/internal/api/sgroups"
	model "github.com/H-BF/sgroups/internal/models/sgroups"

	"github.com/H-BF/corlib/pkg/parallel"
	sgAPI "github.com/H-BF/protos/pkg/api/sgroups"
	"github.com/ahmetb/go-linq/v3"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type (
	// SgFrom ...
	SgFrom = struct {
		SgName
		Transport model.NetworkTransport
	}
	// SgTo ...
	SgTo = map[SgName]RulePorts
	// SgRules ...
	SgRules = map[SgFrom]SgTo

	// RulePorts ...
	RulePorts = []model.SGRulePorts

	// RuleIdentity -
	RuleIdentity = struct {
		SgFrom, SgTo string
		Proto        model.NetworkTransport
	}

	// RulesOutTempalte -
	RulesOutTemplate struct {
		SgOut string
		In    []struct {
			Sg    string
			Proto model.NetworkTransport
		}
	}

	// RulesInTempalte -
	RulesInTemplate struct {
		SgIn string
		Out  []struct {
			Sg    string
			Proto model.NetworkTransport
		}
	}

	// LocalRules ...
	LocalRules struct {
		SgRules
		LocalSGs LocalSGs
		UsedSGs  map[SgName]*SG
	}

	// SgNetworks ...
	SgNetworks = struct {
		V4, V6 []net.IPNet
	}

	// Sg2Networks ...
	Sg2Networks map[SgName]*SgNetworks
)

// Load ...
func (rules *LocalRules) Load(ctx context.Context, client SGClient, locals LocalSGs) error {
	const api = "LocalRules/Load"

	rules.SgRules = make(SgRules)
	rules.LocalSGs = make(LocalSGs)
	rules.UsedSGs = make(map[SgName]*SG)
	var sgNames []string

	localSgNames := locals.Names()
	if len(localSgNames) == 0 {
		return nil
	}
	reqs := []sgAPI.FindRulesReq{
		{SgFrom: localSgNames}, {SgTo: localSgNames},
	}
	for i := range reqs {
		req, isFrom := &reqs[i], i == 0
		resp, err := client.FindRules(ctx, req)
		if err != nil {
			return errors.WithMessage(err, api)
		}
		for _, protoRule := range resp.GetRules() {
			var rule model.SGRule
			if rule, err = conv.Proto2ModelSGRule(protoRule); err != nil {
				return errors.WithMessage(err, api)
			}
			if isFrom {
				rules.addRule(rule)
				if loc := locals[rule.SgFrom.Name]; loc != nil {
					rules.LocalSGs[rule.SgFrom.Name] = loc
				}
			} else {
				rules.addRule(rule)
				if loc := locals[rule.SgTo.Name]; loc != nil {
					rules.LocalSGs[rule.SgTo.Name] = loc
				}
			}
			for _, n := range []string{rule.SgFrom.Name, rule.SgTo.Name} {
				if rules.UsedSGs[n] == nil {
					rules.UsedSGs[n] = &SG{Name: n}
					sgNames = append(sgNames, n)
				}
			}
		}
	}

	err := parallel.ExecAbstract(len(sgNames), 7, func(i int) error { //nolint:gomnd
		rq := sgAPI.GetSgSubnetsReq{SgName: sgNames[i]}
		resp, e := client.GetSgSubnets(ctx, &rq)
		if e == nil {
			for _, nw := range resp.GetNetworks() {
				var m model.Network
				m, e = conv.Proto2ModelNetwork(nw)
				if e != nil {
					return e
				}
				o := rules.UsedSGs[sgNames[i]]
				o.Networks = append(o.Networks, m)
			}
		}
		if status.Code(errors.Cause(e)) == codes.NotFound {
			e = nil
		}
		return e
	})
	return errors.WithMessage(err, api)
}

func (rules *LocalRules) addRule(rule model.SGRule) {
	sgFrom := SgFrom{
		Transport: rule.Transport,
		SgName:    rule.SgFrom.Name,
	}
	sgTo := rules.SgRules[sgFrom]
	if sgTo == nil {
		sgTo = make(SgTo)
		rules.SgRules[sgFrom] = sgTo
	}
	sgTo[rule.SgTo.Name] = rule.Ports
}

// IterateNetworks ...
func (rules LocalRules) IterateNetworks(f func(sgName string, nets []net.IPNet, isV6 bool) error) error {
	type tk = struct {
		sgName string
		v6     bool
	}
	var sg2nws Sg2Networks
	sg2nws.Init(rules)
	seen := make(map[tk]bool)
	send := func(sgName string, isV6 bool, nets []net.IPNet) error {
		k := tk{sgName, isV6}
		if !seen[k] {
			seen[k] = true
			return f(sgName, nets, isV6)
		}
		return nil
	}
	for from, to := range rules.SgRules {
		nw1 := sg2nws[from.SgName]
		for toSg := range to {
			nw2 := sg2nws[toSg]
			if len(nw1.V4) > 0 && len(nw2.V4) > 0 {
				err := send(from.SgName, false, nw1.V4)
				if err == nil {
					err = send(toSg, false, nw2.V4)
				}
				if err != nil {
					return err
				}
			}
			if len(nw1.V6) > 0 && len(nw2.V6) > 0 {
				err := send(from.SgName, true, nw1.V6)
				if err == nil {
					err = send(toSg, true, nw2.V6)
				}
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// TemplatesOutRules -
func (rules LocalRules) TemplatesOutRules() []RulesOutTemplate {
	type groupped = struct {
		Sg    string
		Proto model.NetworkTransport
	}
	var data []RuleIdentity
	for from, to := range rules.SgRules {
		if rules.LocalSGs[from.SgName] != nil {
			id := RuleIdentity{SgFrom: from.SgName, Proto: from.Transport}
			for toSg := range to {
				id.SgTo = toSg
				data = append(data, id)
			}
		}
	}
	groups := make([]linq.Group, 0, len(data))
	linq.From(data).
		GroupByT(
			func(o RuleIdentity) string {
				return o.SgFrom
			},
			func(o RuleIdentity) groupped {
				return groupped{Sg: o.SgTo, Proto: o.Proto}
			},
		).ToSlice(&groups)
	ret := make([]RulesOutTemplate, 0, len(groups))
	for _, g := range groups {
		item := RulesOutTemplate{SgOut: g.Key.(string)}
		if len(g.Group) > 0 {
			linq.From(g.Group).Distinct().ToSlice(&item.In)
		}
		ret = append(ret, item)
	}
	return ret
}

// TemplatesInRules -
func (rules LocalRules) TemplatesInRules() []RulesInTemplate {
	type groupped = struct {
		Sg    string
		Proto model.NetworkTransport
	}
	var data []RuleIdentity
	for from, to := range rules.SgRules {
		id := RuleIdentity{SgFrom: from.SgName, Proto: from.Transport}
		for toSg := range to {
			if rules.LocalSGs[toSg] != nil {
				id.SgTo = toSg
				data = append(data, id)
			}
		}
	}
	groups := make([]linq.Group, 0, len(data))
	linq.From(data).
		GroupByT(
			func(o RuleIdentity) string {
				return o.SgTo
			},
			func(o RuleIdentity) groupped {
				return groupped{Sg: o.SgFrom, Proto: o.Proto}
			},
		).ToSlice(&groups)
	ret := make([]RulesInTemplate, 0, len(groups))
	for _, g := range groups {
		item := RulesInTemplate{SgIn: g.Key.(string)}
		if len(g.Group) > 0 {
			linq.From(g.Group).Distinct().ToSlice(&item.Out)
		}
		ret = append(ret, item)
	}
	return ret
}

// Init ...
func (sg2nws *Sg2Networks) Init(locals LocalRules) {
	*sg2nws = make(Sg2Networks)
	for _, sg := range locals.UsedSGs {
		var nws SgNetworks
		nws.V4, nws.V6 = separateNetworks(sg.Networks)
		(*sg2nws)[sg.Name] = &nws
	}
}
