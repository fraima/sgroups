package internal

import (
	"context"
	"fmt"
	"net"

	"github.com/H-BF/protos/pkg/api/common"
	sgroupsAPI "github.com/H-BF/protos/pkg/api/sgroups"
	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	// RcNetworks -
	RcNetworks = SGroupsProvider + "_networks"

	// RcLabelNetworks -
	RcLabelNetworks = "networks"

	// RcLabelItems -
	RcLabelItems = "items"

	// RcLabelName -
	RcLabelName = "name"

	// RcLabelCIDR -
	RcLabelCIDR = "cidr"
)

/*// resource skeleton
items:
- name: nw1
  cidr: 1.1.1.0/24
- name: nw2
  cidr: 2.2.2.0/24
*/

// SGroupsRcNetworks networks resource
func SGroupsRcNetworks() *schema.Resource {
	return &schema.Resource{
		Description:   fmt.Sprintf("represents networks resource in '%s' provider", SGroupsProvider),
		CreateContext: networksC,
		UpdateContext: networksU,
		DeleteContext: networksD,
		ReadContext:   networksR,
		Schema: map[string]*schema.Schema{
			RcLabelItems: {
				Optional:    true,
				Description: "newtwork list",
				Type:        schema.TypeList,
				Elem: &schema.Resource{
					Description: "network element",
					Schema: map[string]*schema.Schema{
						RcLabelName: {
							Description: "network name",
							Type:        schema.TypeString,
							Required:    true,
						},
						RcLabelCIDR: {
							Description: "network in 'CIDR' format",
							Type:        schema.TypeString,
							Required:    true,
							ValidateDiagFunc: func(v interface{}, _ cty.Path) diag.Diagnostics {
								s := v.(string)
								if _, _, err := net.ParseCIDR(s); err != nil {
									return diag.Errorf("bad CIDR '%s': %s", s, err.Error())
								}
								return nil
							},
						},
					},
				},
			},
		},
	}
}

type crudNetworks = listedRcCRUD[sgroupsAPI.Network]

func networksR(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
	var h listedResource[sgroupsAPI.Network]
	h.init("", ";")
	var req sgroupsAPI.ListNetworksReq
	items, _ := rd.Get(RcLabelItems).([]any)
	if err := h.addlist(items, tf2protoNetwork); err != nil {
		return diag.FromErr(err)
	}
	if len(h.mapped) == 0 {
		rd.SetId(noneID)
		return nil
	}
	h.walk(func(k string, _ *sgroupsAPI.Network) bool {
		req.NeteworkNames = append(req.NeteworkNames, k)
		h.set(k, nil)
		return true
	})
	resp, err := i.(SGClient).ListNetworks(ctx, &req)
	if err != nil {
		return diag.FromErr(err)
	}
	for _, n := range resp.GetNetworks() {
		h.set(n.GetName(), n)
	}
	items = items[:0]
	h.walk(func(k string, nw *sgroupsAPI.Network) bool {
		if nw != nil {
			items = append(items, map[string]any{
				RcLabelName: nw.GetName(),
				RcLabelCIDR: nw.GetNetwork().GetCIDR(),
			})
		}
		return true
	})
	rd.SetId(h.id(noneID))
	return diag.FromErr(
		rd.Set(RcLabelItems, items),
	)
}

func networksC(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
	crud := crudNetworks{tf2proto: tf2protoNetwork, labelItems: RcLabelItems, client: i.(SGClient)}
	return diag.FromErr(crud.create(ctx, rd))
}

func networksU(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
	crud := crudNetworks{tf2proto: tf2protoNetwork, labelItems: RcLabelItems, client: i.(SGClient)}
	return diag.FromErr(crud.update(ctx, rd))
}

func networksD(ctx context.Context, rd *schema.ResourceData, i interface{}) diag.Diagnostics {
	crud := crudNetworks{tf2proto: tf2protoNetwork, labelItems: RcLabelItems, client: i.(SGClient)}
	return diag.FromErr(crud.delete(ctx, rd))
}

func tf2protoNetwork(raw any) (string, *sgroupsAPI.Network, error) {
	it := raw.(map[string]interface{})
	name := it[RcLabelName].(string)
	return name, &sgroupsAPI.Network{
		Name: name,
		Network: &common.Networks_NetIP{
			CIDR: it[RcLabelCIDR].(string),
		},
	}, nil
}
