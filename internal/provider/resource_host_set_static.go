package provider

import (
	"context"
	"net/http"

	"github.com/hashicorp/boundary/api"
	"github.com/hashicorp/boundary/api/hostsets"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	hostSetHostIdsKey = "host_ids"
	hostSetTypeStatic = "static"
)

func resourceHostSet() *schema.Resource {
	return &schema.Resource{
		DeprecationMessage: "Deprecated: use `resource_host_set_static` instead.",
		Description:        "Deprecated: use `resource_host_set_static` instead.",

		CreateContext: resourceHostSetStaticCreate,
		ReadContext:   resourceHostSetStaticRead,
		UpdateContext: resourceHostSetStaticUpdate,
		DeleteContext: resourceHostSetStaticDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			IDKey: {
				Description: "The ID of the host set.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			NameKey: {
				Description: "The host set name. Defaults to the resource name.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			DescriptionKey: {
				Description: "The host set description.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			TypeKey: {
				Description: "The type of host set",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			HostCatalogIdKey: {
				Description: "The catalog for the host set.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			hostSetHostIdsKey: {
				Description: "The list of host IDs contained in this set.",
				Type:        schema.TypeSet,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceHostSetStatic() *schema.Resource {
	return &schema.Resource{
		Description: "The host_set_static resource allows you to configure a Boundary host set. Host sets are " +
			"always part of a host catalog, so a host catalog resource should be used inline or you " +
			"should have the host catalog ID in hand to successfully configure a host set.",

		CreateContext: resourceHostSetStaticCreate,
		ReadContext:   resourceHostSetStaticRead,
		UpdateContext: resourceHostSetStaticUpdate,
		DeleteContext: resourceHostSetStaticDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			IDKey: {
				Description: "The ID of the host set.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			NameKey: {
				Description: "The host set name. Defaults to the resource name.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			DescriptionKey: {
				Description: "The host set description.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			TypeKey: {
				Description: "The type of host set",
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Default:     hostSetTypeStatic,
			},
			HostCatalogIdKey: {
				Description: "The catalog for the host set.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			hostSetHostIdsKey: {
				Description: "The list of host IDs contained in this set.",
				Type:        schema.TypeSet,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func setFromHostSetStaticResponseMap(d *schema.ResourceData, raw map[string]interface{}) error {
	if err := d.Set(NameKey, raw["name"]); err != nil {
		return err
	}
	if err := d.Set(DescriptionKey, raw["description"]); err != nil {
		return err
	}
	if err := d.Set(HostCatalogIdKey, raw["host_catalog_id"]); err != nil {
		return err
	}
	if err := d.Set(TypeKey, raw["type"]); err != nil {
		return err
	}
	if err := d.Set(hostSetHostIdsKey, raw["host_ids"]); err != nil {
		return err
	}
	d.SetId(raw["id"].(string))
	return nil
}

func resourceHostSetStaticCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	md := meta.(*metaData)

	var hostsetHostCatalogId string
	if hostsetHostCatalogIdVal, ok := d.GetOk(HostCatalogIdKey); ok {
		hostsetHostCatalogId = hostsetHostCatalogIdVal.(string)
	} else {
		return diag.Errorf("no host catalog ID provided")
	}

	var hostIds []string
	if hostIdsVal, ok := d.GetOk(hostSetHostIdsKey); ok {
		list := hostIdsVal.(*schema.Set).List()
		hostIds = make([]string, 0, len(list))
		for _, i := range list {
			hostIds = append(hostIds, i.(string))
		}
	}

	opts := []hostsets.Option{}

	var typeStr string
	if typeVal, ok := d.GetOk(TypeKey); ok {
		typeStr = typeVal.(string)
	} else {
		return diag.Errorf("no type provided")
	}
	switch typeStr {
	// NOTE: When other types are added, ensure they don't accept hostSetIds if
	// it's not allowed
	case hostSetTypeStatic:
	default:
		return diag.Errorf("invalid type provided")
	}

	nameVal, ok := d.GetOk(NameKey)
	if ok {
		nameStr := nameVal.(string)
		opts = append(opts, hostsets.WithName(nameStr))
	}

	descVal, ok := d.GetOk(DescriptionKey)
	if ok {
		descStr := descVal.(string)
		opts = append(opts, hostsets.WithDescription(descStr))
	}

	hsClient := hostsets.NewClient(md.client)

	hscr, err := hsClient.Create(ctx, hostsetHostCatalogId, opts...)
	if err != nil {
		return diag.Errorf("error creating host set: %v", err)
	}
	if hscr == nil {
		return diag.Errorf("nil host set after create")
	}
	raw := hscr.GetResponse().Map

	if hostIds != nil {
		hsshr, err := hsClient.SetHosts(ctx, hscr.Item.Id, hscr.Item.Version, hostIds)
		if err != nil {
			return diag.Errorf("error setting hosts on host set: %v", err)
		}
		if hsshr == nil {
			return diag.Errorf("nil host set after setting hosts")
		}
		raw = hsshr.GetResponse().Map
	}

	if err := setFromHostSetStaticResponseMap(d, raw); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceHostSetStaticRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	md := meta.(*metaData)
	hsClient := hostsets.NewClient(md.client)

	hsrr, err := hsClient.Read(ctx, d.Id())
	if err != nil {
		if apiErr := api.AsServerError(err); apiErr != nil && apiErr.Response().StatusCode() == http.StatusNotFound {
			d.SetId("")
			return nil
		}
		return diag.Errorf("error reading host set: %v", err)
	}
	if hsrr == nil {
		return diag.Errorf("host set nil after read")
	}

	if err := setFromHostSetStaticResponseMap(d, hsrr.GetResponse().Map); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceHostSetStaticUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	md := meta.(*metaData)
	hsClient := hostsets.NewClient(md.client)

	opts := []hostsets.Option{}

	var name *string
	if d.HasChange(NameKey) {
		opts = append(opts, hostsets.DefaultName())
		nameVal, ok := d.GetOk(NameKey)
		if ok {
			nameStr := nameVal.(string)
			name = &nameStr
			opts = append(opts, hostsets.WithName(nameStr))
		}
	}

	var desc *string
	if d.HasChange(DescriptionKey) {
		opts = append(opts, hostsets.DefaultDescription())
		descVal, ok := d.GetOk(DescriptionKey)
		if ok {
			descStr := descVal.(string)
			desc = &descStr
			opts = append(opts, hostsets.WithDescription(descStr))
		}
	}

	if len(opts) > 0 {
		opts = append(opts, hostsets.WithAutomaticVersioning(true))
		_, err := hsClient.Update(ctx, d.Id(), 0, opts...)
		if err != nil {
			return diag.Errorf("error updating host set: %v", err)
		}
	}

	if d.HasChange(NameKey) {
		if err := d.Set(NameKey, name); err != nil {
			return diag.FromErr(err)
		}
	}
	if d.HasChange(DescriptionKey) {
		if err := d.Set(DescriptionKey, desc); err != nil {
			return diag.FromErr(err)
		}
	}

	// The above call may not actually happen, so we use d.Id() and automatic
	// versioning here
	if d.HasChange(hostSetHostIdsKey) {
		var hostIds []string
		if hostIdsVal, ok := d.GetOk(hostSetHostIdsKey); ok {
			hosts := hostIdsVal.(*schema.Set).List()
			for _, host := range hosts {
				hostIds = append(hostIds, host.(string))
			}
		}
		_, err := hsClient.SetHosts(ctx, d.Id(), 0, hostIds, hostsets.WithAutomaticVersioning(true))
		if err != nil {
			return diag.Errorf("error updating hosts in host set: %v", err)
		}
		if err := d.Set(hostSetHostIdsKey, hostIds); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourceHostSetStaticDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	md := meta.(*metaData)
	hsClient := hostsets.NewClient(md.client)

	_, err := hsClient.Delete(ctx, d.Id())
	if err != nil {
		return diag.Errorf("error deleting host set: %s", err.Error())
	}

	return nil
}
