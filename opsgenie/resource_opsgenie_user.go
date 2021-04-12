package opsgenie

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/opsgenie/opsgenie-go-sdk-v2/user"

	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceOpsGenieUser() *schema.Resource {
	return &schema.Resource{
		Create: resourceOpsGenieUserCreate,
		Read:   handleNonExistentResource(resourceOpsGenieUserRead),
		Update: resourceOpsGenieUserUpdate,
		Delete: resourceOpsGenieUserDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"username": {
				Type:         schema.TypeString,
				ForceNew:     true,
				Required:     true,
				ValidateFunc: validateOpsGenieUserUsername,
			},
			"full_name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validateOpsGenieUserFullName,
			},
			"role": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validateOpsGenieUserRole,
			},
			"locale": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"timezone": {
				Type:             schema.TypeString,
				Optional:         true,
				Computed:         true,
				DiffSuppressFunc: checkTimeZoneDiff,
			},
			"tags": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Set: schema.HashString,
			},
			"user_address": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"country": {
							Type:     schema.TypeString,
							Required: true,
						},
						"state": {
							Type:     schema.TypeString,
							Required: true,
						},
						"city": {
							Type:     schema.TypeString,
							Required: true,
						},
						"line": {
							Type:     schema.TypeString,
							Required: true,
						},
						"zipcode": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			"user_details": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"skype_username": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func checkOptionalProperties(d *schema.ResourceData) *user.User {
	var userAddrStruct user.UserAddress
	if _, exists := d.GetOk("user_address"); exists {
		address := expandOpsGenieUserAddress(d)
		userAddrStruct.State = address["state"]
		userAddrStruct.City = address["city"]
		userAddrStruct.Country = address["country"]
		userAddrStruct.Line = address["line"]
		userAddrStruct.ZipCode = address["zipcode"]
	}

	u := &user.User{
		Username: d.Get("username").(string),
		FullName: d.Get("full_name").(string),
		Role: &user.UserRole{
			RoleName: d.Get("role").(string),
		},
		UserAddress: &userAddrStruct,
	}

	if value, exists := d.GetOk("locale"); exists {
		u.Locale = value.(string)
	}
	if value, exists := d.GetOk("timezone"); exists {
		u.TimeZone = value.(string)
	}
	if value, exists := d.GetOk("tags"); exists {
		u.Tags = expandOpsGenieUsertags(value.(*schema.Set))
	}

	if _, exists := d.GetOk("user_details"); exists {
		u.Details = expandOpsGenieUserDetails(d)
	}
	return u
}

func checkTimeZoneDiff(k, old, new string, d *schema.ResourceData) bool {
	locationOld, errOld := time.LoadLocation(old)
	if errOld != nil {
		return false
	}
	locationNew, errNew := time.LoadLocation(new)
	if errNew != nil {
		return false
	}
	now := time.Now()
	timeOld := now.In(locationOld)
	timeNew := now.In(locationNew)
	return timeOld.Format(time.ANSIC) == timeNew.Format(time.ANSIC)
}

func expandOpsGenieUsertags(input *schema.Set) []string {
	output := make([]string, 0)

	if input == nil {
		return output
	}

	for _, v := range input.List() {
		output = append(output, v.(string))
	}
	return output
}

func expandOpsGenieUserAddress(d *schema.ResourceData) map[string]string {
	input := d.Get("user_address").([]interface{})
	output := make(map[string]string)

	if input == nil {
		return output
	}

	for _, v := range input {
		config := v.(map[string]interface{})

		output["country"] = config["country"].(string)
		output["state"] = config["state"].(string)
		output["city"] = config["city"].(string)
		output["line"] = config["line"].(string)
		output["zipcode"] = config["zipcode"].(string)

	}
	return output
}

func expandOpsGenieUserDetails(d *schema.ResourceData) map[string][]string {
	input := d.Get("user_details").(map[string]interface{})
	output := make(map[string][]string)

	if input == nil {
		return output
	}

	for k, v := range input {
		output[k] = strings.Split(v.(string), ",")
	}

	return output
}

func resourceOpsGenieUserCreate(d *schema.ResourceData, meta interface{}) error {
	client, err := user.NewClient(meta.(*OpsgenieClient).client.Config)
	if err != nil {
		return err
	}

	u := checkOptionalProperties(d)

	var skypeUsername string
	if value, exists := d.GetOk("user_details"); exists {
		skypeUsername = value.(string)
	}

	createRequest := &user.CreateRequest{
		Username: u.Username,
		FullName: u.FullName,
		Role: &user.UserRoleRequest{
			RoleName: u.Role.RoleName,
		},
		Locale:   u.Locale,
		TimeZone: u.TimeZone,
		Tags:     u.Tags,
		UserAddressRequest: &user.UserAddressRequest{
			State:   u.UserAddress.State,
			City:    u.UserAddress.City,
			Line:    u.UserAddress.Line,
			ZipCode: u.UserAddress.ZipCode,
		},
		Details:       u.Details,
		SkypeUsername: skypeUsername,
	}

	log.Printf("[INFO] Creating OpsGenie user '%s'", u.Username)
	result, err := client.Create(context.Background(), createRequest)
	if err != nil {
		return err
	}

	d.SetId(result.Id)

	return resourceOpsGenieUserRead(d, meta)
}

func resourceOpsGenieUserRead(d *schema.ResourceData, meta interface{}) error {
	client, err := user.NewClient(meta.(*OpsgenieClient).client.Config)
	if err != nil {
		return err
	}
	username := d.Get("username").(string)

	log.Printf("[INFO] Reading OpsGenie user '%s'", username)

	usr, err := client.Get(context.Background(), &user.GetRequest{
		Identifier: d.Id(),
	})
	if err != nil {
		return err
	}
	d.Set("username", usr.Username)
	d.Set("full_name", usr.FullName)
	d.Set("role", usr.Role.RoleName)
	d.Set("locale", usr.Locale)
	d.Set("timezone", usr.TimeZone)
	d.Set("tags", usr.Tags)
	if usr.UserAddress != nil && usr.UserAddress.Country != "" {
		d.Set("user_address", flattenUserAddress(usr.UserAddress))
	}
	//d.Set("user_details", usr.Details) TODO FIX
	d.Set("skype_username", usr.SkypeUsername)

	return nil
}

func resourceOpsGenieUserUpdate(d *schema.ResourceData, meta interface{}) error {
	client, err := user.NewClient(meta.(*OpsgenieClient).client.Config)
	if err != nil {
		return err
	}
	u := checkOptionalProperties(d)
	skypeUsername := d.Get("skype_username").(string)

	log.Printf("[INFO] Updating OpsGenie user '%s'", u.Username)

	updateRequest := &user.UpdateRequest{
		Identifier: d.Id(),
		Username:   u.Username,
		FullName:   u.FullName,
		Role: &user.UserRoleRequest{
			RoleName: u.Role.RoleName,
		},
		Locale:   u.Locale,
		TimeZone: u.TimeZone,
		Tags:     u.Tags,
		UserAddressRequest: &user.UserAddressRequest{
			Country: u.UserAddress.Country,
			State:   u.UserAddress.State,
			City:    u.UserAddress.City,
			Line:    u.UserAddress.Line,
			ZipCode: u.UserAddress.ZipCode,
		},
		Details:       u.Details,
		SkypeUsername: skypeUsername,
	}
	_, err = client.Update(context.Background(), updateRequest)
	if err != nil {
		return err
	}

	return resourceOpsGenieUserRead(d, meta)
}

func resourceOpsGenieUserDelete(d *schema.ResourceData, meta interface{}) error {
	log.Printf("[INFO] Deleting OpsGenie user '%s'", d.Get("username").(string))
	client, err := user.NewClient(meta.(*OpsgenieClient).client.Config)
	if err != nil {
		return err
	}
	deleteRequest := &user.DeleteRequest{
		Identifier: d.Id(),
	}

	_, err = client.Delete(context.Background(), deleteRequest)
	if err != nil {
		return err
	}

	return nil
}

func validateOpsGenieUserUsername(v interface{}, k string) (ws []string, errors []error) {
	value := v.(string)

	if len(value) >= 100 {
		errors = append(errors, fmt.Errorf("%q cannot be longer than 100 characters: %q %d", k, value, len(value)))
	}

	if value != strings.ToLower(value) {
		errors = append(errors, fmt.Errorf("%v contains uppercase characters, only lowercase characters are allowed: %q", k, value))
	}

	return
}

func validateOpsGenieUserFullName(v interface{}, k string) (ws []string, errors []error) {
	value := v.(string)

	if len(value) >= 512 {
		errors = append(errors, fmt.Errorf("%q cannot be longer than 512 characters: %q %d", k, value, len(value)))
	}

	return
}

func validateOpsGenieUserRole(v interface{}, k string) (ws []string, errors []error) {
	value := v.(string)

	if len(value) >= 512 {
		errors = append(errors, fmt.Errorf("%q cannot be longer than 512 characters: %q %d", k, value, len(value)))
	}
	return
}

func flattenUserAddress(addr *user.UserAddress) []map[string]interface{} {
	return []map[string]interface{}{{
		"country": addr.Country,
		"state":   addr.State,
		"city":    addr.City,
		"line":    addr.Line,
		"zipcode": addr.ZipCode,
	}}
}
