package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/oracle/oci-go-sdk/v65/bastion"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

func initialize_oci_clients() (identity.IdentityClient, bastion.BastionClient) {
	config := common.DefaultConfigProvider() // TODO: Flex on OCI profile

	identity_client, identity_err := identity.NewIdentityClientWithConfigurationProvider(config)
	if identity_err != nil {
		panic(identity_err)
	}

	bastion_client, err := bastion.NewBastionClientWithConfigurationProvider(config)
	if err != nil {
		panic(err)
	}

	return identity_client, bastion_client
}

func check_tenancy(client identity.IdentityClient) string {
	tenant_id, exists := os.LookupEnv("OCI_CLI_TENANCY")
	if !exists {
		fmt.Println("OCI_CLI_TENANCY is not set. OCI_CLI_TENANCY env var must be set. Exiting program...")
		os.Exit(1)
	}

	print_tenancy_name(tenant_id, client)

	return tenant_id
}

func print_tenancy_name(tenant_id string, client identity.IdentityClient) {
	response, err := client.GetTenancy(context.Background(), identity.GetTenancyRequest{TenancyId: &tenant_id})
	if err != nil {
		panic(err)
	}

	fmt.Println("Current tenant: " + *response.Tenancy.Name)
}

func get_compartment_info(tenant_id string, client identity.IdentityClient) map[string]string {
	response, err := client.ListCompartments(context.Background(), identity.ListCompartmentsRequest{CompartmentId: &tenant_id})
	if err != nil {
		panic(err)
	}

	compartment_info := make(map[string]string)

	for _, item := range response.Items {
		compartment_info[*item.Name] = *item.Id
	}

	return compartment_info
}

func get_bastion_info(compartment_id string, client bastion.BastionClient) map[string]string {
	response, err := client.ListBastions(context.Background(), bastion.ListBastionsRequest{CompartmentId: &compartment_id})
	if err != nil {
		//Something happened
		panic(err)
	}

	bastion_info := make(map[string]string)

	for _, item := range response.Items {
		bastion_info[*item.Name] = *item.Id
	}

	return bastion_info
}

func get_bastion(bastion_name string, bastion_id string, client bastion.BastionClient) {
	fmt.Println("\nGetting bastion for: " + bastion_name + " (" + bastion_id + ")")
	_, err := client.GetBastion(context.Background(), bastion.GetBastionRequest{BastionId: &bastion_id})
	if err != nil {
		//Something happened
		fmt.Println("\nERROR on GetBastion")
		panic(err)
	} else {
		fmt.Println("\nSUCCESS on GetBastion\n")
	}
}

func create_session(bastion_id string, client bastion.BastionClient, target_instance string, target_ip string, public_key_content string) *string {
	req := bastion.CreateSessionRequest{CreateSessionDetails: bastion.CreateSessionDetails{
		BastionId:   &bastion_id,
		DisplayName: common.String("OCIBastionSession"),
		KeyDetails:  &bastion.PublicKeyDetails{PublicKeyContent: &public_key_content},
		// KeyType:             bastion.CreateSessionDetailsKeyTypePub,
		SessionTtlInSeconds: common.Int(1800),
		TargetResourceDetails: bastion.CreateManagedSshSessionTargetResourceDetails{
			TargetResourceId:                      &target_instance,
			TargetResourceOperatingSystemUserName: common.String("cloud-user"),
			TargetResourcePort:                    common.Int(22),
			TargetResourcePrivateIpAddress:        &target_ip}}}

	fmt.Println("\nCreating session...")
	response, err := client.CreateSession(context.Background(), req)
	if err != nil {
		panic(err)
	}

	fmt.Println("\nCreateSessionResponse")
	fmt.Println(response)

	sessionId := response.Session.Id
	fmt.Println("\nSession ID: ")
	fmt.Println(*sessionId)

	return sessionId
}

func check_session(client bastion.BastionClient, sessionId *string) bastion.SessionLifecycleStateEnum { // TODO: TEST
	get_session_response, err := client.GetSession(context.Background(), bastion.GetSessionRequest{SessionId: sessionId}) // TODO: TEST
	if err != nil {
		panic(err)
	}

	fmt.Println("GetSessionResponse")
	fmt.Println(get_session_response.Session)

	fmt.Println("\nEndpoint")
	fmt.Println(client.Endpoint())

	fmt.Println("\nLifecycleState")
	fmt.Println(get_session_response.LifecycleState)

	return get_session_response.LifecycleState
}

func main() {
	flagListCompartments := flag.Bool("list-compartments", false, "list compartments")
	flagCompartmentName := flag.String("c", "", "compartment name")
	flagListBastions := flag.Bool("list-bastions", false, "list bastions")
	flagBastionName := flag.String("b", "", "bastion name")
	flagInstanceId := flag.String("o", "", "instance ID of host to connect to")
	flagInstanceIp := flag.String("i", "", "instance IP address of host to connect to")
	flagSessionId := flag.String("s", "", "Session ID to check for")

	flag.Parse()

	identity_client, bastion_client := initialize_oci_clients()

	tenant_id := check_tenancy(identity_client)

	compartments := get_compartment_info(tenant_id, identity_client)

	if *flagListCompartments {
		fmt.Println("\nCompartments")

		for compartment := range compartments {
			println(compartment) // TODO: these need to be sorted
		}

		os.Exit(0)
	}

	if *flagCompartmentName == "" {
		fmt.Println("Must pass compartment name with -c")
		os.Exit(1)
	}

	compartment_id := compartments[*flagCompartmentName]
	fmt.Println("\n" + *flagCompartmentName + "'s compartment ID is " + compartment_id)

	bastions := get_bastion_info(compartment_id, bastion_client)
	if *flagListBastions {
		fmt.Println("\nBastions in compartment " + *flagCompartmentName)
		for bastion_name := range bastions {
			fmt.Println(bastion_name)
		}

		os.Exit(0)
	}

	bastion_id := bastions[*flagBastionName]
	get_bastion(*flagBastionName, bastion_id, bastion_client)

	// TODO: Consider interface for SSH private key
	public_key_content, exists := os.LookupEnv("OCI_BASTION_SSH_KEY")
	if !exists {
		fmt.Println("OCI_BASTION_SSH_KEY is not set. OCI_BASTION_SSH_KEY env var must be set. Exiting program...")
		os.Exit(1)
	}

	var sessionId *string
	if *flagSessionId == "" {
		sessionId = create_session(bastion_id, bastion_client, *flagInstanceId, *flagInstanceIp, public_key_content)
	} else {
		fmt.Println("Session ID passed, checking session...")
		sessionId = flagSessionId
		session_state := check_session(bastion_client, sessionId)
		fmt.Println(session_state)

		if session_state == "ACTIVE" {
			printSshCommand(bastion_client, sessionId, flagInstanceIp)
		}

		os.Exit(0)
	}

	session_state := check_session(bastion_client, sessionId)

	for session_state != "ACTIVE" {
		fmt.Println("\nSession not yet active")
		fmt.Println("State: " + session_state)
		fmt.Println("\nWaiting...")
		time.Sleep(10 * time.Second)
		session_state = check_session(bastion_client, sessionId)
		// session_state = check_session(bastion_client, sessionId) // TODO: TEST
	}

	printSshCommand(bastion_client, sessionId, flagInstanceIp)
}

func printSshCommand(bastion_client bastion.BastionClient, sessionId *string, flagInstanceIp *string) {
	bastion_endpoint_url, err := url.Parse(bastion_client.Endpoint())
	if err != nil {
		panic(err)
	}

	sessionIdStr := *sessionId
	jumpbox := sessionIdStr + "@host." + bastion_endpoint_url.Host

	fmt.Println("\nssh -i \"<private key file>\" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \\")
	fmt.Println("-o ProxyCommand='ssh -i \"<private key file>\" -W %h:%p -p 22 " + jumpbox + "' \\")
	fmt.Println("-P 22 cloud-user@" + *flagInstanceIp)
}
