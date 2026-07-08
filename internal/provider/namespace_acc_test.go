package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"tpuf": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	if os.Getenv("TURBOPUFFER_API_KEY") == "" {
		t.Skip("TURBOPUFFER_API_KEY must be set for acceptance tests")
	}
	if os.Getenv("TURBOPUFFER_REGION") == "" {
		t.Skip("TURBOPUFFER_REGION must be set for acceptance tests")
	}
}

func TestAccNamespaceResource_basic(t *testing.T) {
	name := fmt.Sprintf("tf-acc-test-%s", acctest.RandString(8))
	resourceName := "tpuf_namespace.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccNamespaceConfig(name, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", name),
					resource.TestCheckResourceAttr(resourceName, "schema.title.type", "string"),
					resource.TestCheckResourceAttr(resourceName, "schema.title.filterable", "true"),
					resource.TestCheckResourceAttrSet(resourceName, "approx_row_count"),
				),
			},
			{
				ResourceName:                         resourceName,
				ImportState:                          true,
				ImportStateId:                        name,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
			{
				// Update: flip filterable off.
				Config: testAccNamespaceConfig(name, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "schema.title.filterable", "false"),
				),
			},
		},
	})
}

func testAccNamespaceConfig(name string, filterable bool) string {
	return fmt.Sprintf(`
resource "tpuf_namespace" "test" {
  name = %[1]q

  schema = {
    title = {
      type       = "string"
      filterable = %[2]t
    }
  }
}
`, name, filterable)
}
