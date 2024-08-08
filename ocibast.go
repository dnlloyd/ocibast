// TODOs:
//   - TODO: 2. Convert session and bastion to objects
package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/oracle/oci-go-sdk/v65/bastion"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

const logLevel = "INFO"

type SessionInfo struct {
	state bastion.SessionLifecycleStateEnum
	ip    string
	user  string
	port  int
}

func initializeOciClients() (identity.IdentityClient, bastion.BastionClient) {
	config := common.DefaultConfigProvider() // TODO: Flex on OCI profile

	identityClient, identityErr := identity.NewIdentityClientWithConfigurationProvider(config)
	if identityErr != nil {
		panic(identityErr)
	}

	bastionClient, err := bastion.NewBastionClientWithConfigurationProvider(config)
	if err != nil {
		panic(err)
	}

	return identityClient, bastionClient
}

func checkTenancy(client identity.IdentityClient) string {
	tenantId, exists := os.LookupEnv("OCI_CLI_TENANCY")
	if !exists {
		fmt.Println("OCI_CLI_TENANCY is not set. OCI_CLI_TENANCY env var must be set. Exiting program...")
		os.Exit(1)
	}

	printTenancyName(tenantId, client)

	return tenantId
}

func printTenancyName(tenantId string, client identity.IdentityClient) {
	response, err := client.GetTenancy(context.Background(), identity.GetTenancyRequest{TenancyId: &tenantId})
	if err != nil {
		panic(err)
	}

	fmt.Println("\nCurrent tenant: " + *response.Tenancy.Name)
}

func getCompartmentInfo(tenantId string, client identity.IdentityClient) map[string]string {
	response, err := client.ListCompartments(context.Background(), identity.ListCompartmentsRequest{CompartmentId: &tenantId})
	if err != nil {
		panic(err)
	}

	compartmentInfo := make(map[string]string)

	for _, item := range response.Items {
		compartmentInfo[*item.Name] = *item.Id
	}

	return compartmentInfo
}

func getBastionInfo(compartmentId string, client bastion.BastionClient) map[string]string {
	response, err := client.ListBastions(context.Background(), bastion.ListBastionsRequest{CompartmentId: &compartmentId})
	if err != nil {
		panic(err)
	}

	bastionInfo := make(map[string]string)

	for _, item := range response.Items {
		bastionInfo[*item.Name] = *item.Id
	}

	return bastionInfo
}

func getBastion(bastionName string, bastionId string, client bastion.BastionClient) {
	if logLevel == "DEBUG" {
		fmt.Println("\nGetting bastion for: " + bastionName + " (" + bastionId + ")")
	}

	_, err := client.GetBastion(context.Background(), bastion.GetBastionRequest{BastionId: &bastionId})
	if err != nil {
		panic(err)
	}
}

func createSession(bastionId string, client bastion.BastionClient, targetInstance string, targetIp string, publicKeyContent string, sshUser string, sshPort int) *string {
	req := bastion.CreateSessionRequest{
		CreateSessionDetails: bastion.CreateSessionDetails{
			BastionId:           &bastionId,
			DisplayName:         common.String("OCIBastionSession"), // TODO: Maybe set this programmatically
			KeyDetails:          &bastion.PublicKeyDetails{PublicKeyContent: &publicKeyContent},
			SessionTtlInSeconds: common.Int(1800),
			TargetResourceDetails: bastion.CreateManagedSshSessionTargetResourceDetails{
				TargetResourceId:                      &targetInstance,
				TargetResourceOperatingSystemUserName: &sshUser,
				TargetResourcePort:                    &sshPort,
				TargetResourcePrivateIpAddress:        &targetIp,
			},
		},
	}

	fmt.Println("\nCreating session...")
	response, err := client.CreateSession(context.Background(), req)
	if err != nil {
		panic(err)
	}

	if logLevel == "DEBUG" {
		fmt.Println("\nCreateSessionResponse")
		fmt.Println(response)
	}

	sessionId := response.Session.Id
	fmt.Println("\nSession ID: ")
	fmt.Println(*sessionId)

	return sessionId
}

func checkSession(client bastion.BastionClient, sessionId *string) SessionInfo {
	response, err := client.GetSession(context.Background(), bastion.GetSessionRequest{SessionId: sessionId})
	if err != nil {
		panic(err)
	}

	if logLevel == "DEBUG" {
		fmt.Println("GetSessionResponse")
		fmt.Println(response.Session)

		fmt.Println("\nEndpoint")
		fmt.Println(client.Endpoint())
	}

	sshSessionTargetResourceDetails := response.Session.TargetResourceDetails.(bastion.ManagedSshSessionTargetResourceDetails)
	ipAddress := sshSessionTargetResourceDetails.TargetResourcePrivateIpAddress
	sshUser := sshSessionTargetResourceDetails.TargetResourceOperatingSystemUserName
	sshPort := sshSessionTargetResourceDetails.TargetResourcePort

	currentSessionInfo := SessionInfo{response.Session.LifecycleState, *ipAddress, *sshUser, *sshPort}

	return currentSessionInfo
}

func printSshCommand(client bastion.BastionClient, sessionId *string, instanceIp *string, sshUser *string, sshPort *int) {
	bastionEndpointUrl, err := url.Parse(client.Endpoint())
	if err != nil {
		panic(err)
	}

	sessionIdStr := *sessionId
	bastionHost := sessionIdStr + "@host." + bastionEndpointUrl.Host

	fmt.Println("\nssh -i \"<private key file>\" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \\")
	fmt.Println("-o ProxyCommand='ssh -i \"<private key file>\" -W %h:%p -p 22 " + bastionHost + "' \\")
	fmt.Println("-P " + strconv.Itoa(*sshPort) + " " + *sshUser + "@" + *instanceIp) // TODO: add vars for user and port
}

func main() {
	flagListCompartments := flag.Bool("list-compartments", false, "list compartments")
	flagCompartmentName := flag.String("c", "", "compartment name")
	flagListBastions := flag.Bool("list-bastions", false, "list bastions")
	flagBastionName := flag.String("b", "", "bastion name")
	flagInstanceId := flag.String("o", "", "instance ID of host to connect to")
	flagInstanceIp := flag.String("i", "", "instance IP address of host to connect to")
	flagSessionId := flag.String("s", "", "Session ID to check for")
	flagSshUser := flag.String("u", "cloud-user", "SSH user")
	flagSshPort := flag.Int("p", 22, "SSH user")
	flag.Parse()

	identityClient, bastionClient := initializeOciClients()

	tenantId := checkTenancy(identityClient)

	compartmentInfo := getCompartmentInfo(tenantId, identityClient)

	if *flagListCompartments {
		fmt.Println("\nCompartments")

		for compartment := range compartmentInfo {
			println(compartment) // TODO: these need to be sorted
		}

		os.Exit(0)
	}

	if *flagCompartmentName == "" {
		fmt.Println("Must pass compartment name with -c")
		os.Exit(1)
	}

	compartmentId := compartmentInfo[*flagCompartmentName]
	if logLevel == "DEBUG" {
		fmt.Println("\n" + *flagCompartmentName + "'s compartment ID is " + compartmentId)
	}

	bastions := getBastionInfo(compartmentId, bastionClient)
	if *flagListBastions {
		fmt.Println("\nBastions in compartment " + *flagCompartmentName)
		for bastionName := range bastions {
			fmt.Println(bastionName)
		}

		os.Exit(0)
	}

	bastionId := bastions[*flagBastionName]
	getBastion(*flagBastionName, bastionId, bastionClient)

	// TODO: implement interface for SSH private key
	publicKeyContent, exists := os.LookupEnv("OCI_BASTION_SSH_KEY")
	if !exists {
		fmt.Println("OCI_BASTION_SSH_KEY is not set. OCI_BASTION_SSH_KEY env var must be set. Exiting program...")
		os.Exit(1)
	}

	var sessionId *string
	if *flagSessionId == "" {
		// No session ID passed, create a new session
		sessionId = createSession(bastionId, bastionClient, *flagInstanceId, *flagInstanceIp, publicKeyContent, *flagSshUser, *flagSshPort)
	} else {
		// Check for existing session by session ID
		fmt.Println("Session ID passed, checking session...")
		sessionId = flagSessionId
		sessionInfo := checkSession(bastionClient, sessionId)

		if sessionInfo.state == "ACTIVE" {
			printSshCommand(bastionClient, sessionId, &sessionInfo.ip, &sessionInfo.user, &sessionInfo.port)
		} else {
			fmt.Println("Session is no longer active. Current state is: " + sessionInfo.state)
		}

		os.Exit(0)
	}

	sessionInfo := checkSession(bastionClient, sessionId)

	for sessionInfo.state != "ACTIVE" {
		fmt.Println("\nSession not yet active")
		fmt.Println("State: " + sessionInfo.state)
		fmt.Println("\nWaiting...")
		time.Sleep(10 * time.Second)
		sessionInfo = checkSession(bastionClient, sessionId)
	}

	printSshCommand(bastionClient, sessionId, flagInstanceIp, flagSshUser, flagSshPort)
}
