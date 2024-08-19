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

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func initializeOciClients() (identity.IdentityClient, bastion.BastionClient) {
	config := common.DefaultConfigProvider() // TODO: Flex on OCI profile

	identityClient, identityErr := identity.NewIdentityClientWithConfigurationProvider(config)
	checkError(identityErr)

	bastionClient, err := bastion.NewBastionClientWithConfigurationProvider(config)
	checkError(err)

	return identityClient, bastionClient
}

func checkTenancy(tenantId string, client identity.IdentityClient) {
	response, err := client.GetTenancy(context.Background(), identity.GetTenancyRequest{TenancyId: &tenantId})
	checkError(err)

	if logLevel == "DEBUG" {
		fmt.Println("\nCurrent tenant: " + *response.Tenancy.Name)
	}
}

func getCompartmentInfo(tenantId string, client identity.IdentityClient) map[string]string {
	response, err := client.ListCompartments(context.Background(), identity.ListCompartmentsRequest{CompartmentId: &tenantId})
	checkError(err)

	compartmentInfo := make(map[string]string)

	for _, item := range response.Items {
		compartmentInfo[*item.Name] = *item.Id
	}

	return compartmentInfo
}

func getBastionInfo(compartmentId string, client bastion.BastionClient) map[string]string {
	response, err := client.ListBastions(context.Background(), bastion.ListBastionsRequest{CompartmentId: &compartmentId})
	checkError(err)

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
	checkError(err)
}

func getSshPubKeyContents(sshPrivateKeyFileLocation string) string {
	homeDir, err := os.UserHomeDir()
	checkError(err)

	if sshPrivateKeyFileLocation == "" {
		sshPrivateKeyFileLocation = homeDir + "/.ssh/id_rsa"
		fmt.Println("\nUsing default SSH identity file at " + sshPrivateKeyFileLocation)
	}

	sshKeyContents, err := os.ReadFile(sshPrivateKeyFileLocation)
	checkError(err)

	return string(sshKeyContents)
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
	checkError(err)

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
	checkError(err)

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

func listActiveSessions(client bastion.BastionClient, bastionId string) {
	response, err := client.ListSessions(context.Background(), bastion.ListSessionsRequest{BastionId: &bastionId})
	checkError(err)

	fmt.Println("\nActive bastion sessions")
	for _, session := range response.Items {
		sshSessionTargetResourceDetails := session.TargetResourceDetails.(bastion.ManagedSshSessionTargetResourceDetails)
		instanceName := sshSessionTargetResourceDetails.TargetResourceDisplayName
		ipAddress := sshSessionTargetResourceDetails.TargetResourcePrivateIpAddress
		instanceID := sshSessionTargetResourceDetails.TargetResourceId

		if session.LifecycleState == "ACTIVE" {
			fmt.Println(*session.DisplayName)
			fmt.Println(*session.Id)
			fmt.Println(*session.TimeCreated)
			fmt.Println(*instanceName)
			fmt.Println(*ipAddress)
			fmt.Println(*instanceID)
			fmt.Println("")
		}
	}
}

func printSshCommands(client bastion.BastionClient, sessionId *string, instanceIp *string, sshUser *string, sshPort *int, sshIdentityFile string) {
	bastionEndpointUrl, err := url.Parse(client.Endpoint())
	checkError(err)

	sessionIdStr := *sessionId
	bastionHost := sessionIdStr + "@host." + bastionEndpointUrl.Host

	fmt.Println("\nTunnel:")
	fmt.Println("ssh -N -L <LOCAL PORT>:" + *instanceIp + ":<DESTINATION PORT> \\")
	fmt.Println("-i " + sshIdentityFile + " -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \\")
	fmt.Println("-o ProxyCommand='ssh -i " + sshIdentityFile + " -W %h:%p -p 22 " + bastionHost + "' \\")
	fmt.Println("-P " + strconv.Itoa(*sshPort) + " " + *sshUser + "@" + *instanceIp)

	fmt.Println("\nSSH:")
	fmt.Println("ssh -i " + sshIdentityFile + " -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \\")
	fmt.Println("-o ProxyCommand='ssh -i " + sshIdentityFile + " -W %h:%p -p 22 " + bastionHost + "' \\")
	fmt.Println("-P " + strconv.Itoa(*sshPort) + " " + *sshUser + "@" + *instanceIp)
}

func main() {
	flagTenancyId := flag.String("t", "", "tenancy ID name")
	flagListCompartments := flag.Bool("list-compartments", false, "list compartments")
	flagCompartmentName := flag.String("c", "", "compartment name")
	flagListBastions := flag.Bool("list-bastions", false, "list bastions")
	flagBastionName := flag.String("b", "", "bastion name")
	flagInstanceId := flag.String("o", "", "instance ID of host to connect to")
	flagInstanceIp := flag.String("i", "", "instance IP address of host to connect to")
	flagSessionId := flag.String("s", "", "Session ID to check for")
	flagListSessions := flag.Bool("list-sessions", false, "list sessions")
	flagSshUser := flag.String("u", "opc", "SSH user")
	flagSshPort := flag.Int("p", 22, "SSH port")
	flagSshPrivateKey := flag.String("k", "", "path to SSH private key (identity file)")
	flagSshPublicKey := flag.String("e", "", "path to SSH public key")

	// Extend flag's default usage function
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()

		fmt.Println("\nCommon command patterns:")
		fmt.Println("List compartments")
		fmt.Println("   ocibast -list-compartments")
		fmt.Println("\nList bastions")
		fmt.Println("   ocibast -c compartment_name -list-bastions")
		fmt.Println("\nCreate bastion session")
		fmt.Println("   ocibast -c compartment_name -b bastion_name -i ip_address -o instance_id -k path_to_ssh_private_key -e path_to_ssh_public_key")
		fmt.Println("\nList active sessions")
		fmt.Println("   ocibast -c mycompartment -b mybastion -list-sessions")
		fmt.Println("\nConnect to an active session")
		fmt.Println("   ocibast -c compartment_name -b bastion_name -k path_to_ssh_private_key -e path_to_ssh_public_key -s session_ocd")

		fmt.Println("\nEnvironment variables:")
		fmt.Println("The following environment variables will override their flag counterparts")
		fmt.Println("   OCI_CLI_TENANCY")
		fmt.Println("   OCI_COMPARTMENT_NAME")
	}

	flag.Parse()

	identityClient, bastionClient := initializeOciClients()

	// Tenancy ID is requred for anything past this point
	var tenantId string
	tenantId, exists := os.LookupEnv("OCI_CLI_TENANCY")
	if !exists {
		if *flagTenancyId == "" {
			fmt.Println("Must pass tenancy ID with -t or set with environment variable OCI_CLI_TENANCY")
			os.Exit(1)
		} else {
			tenantId = *flagTenancyId
		}
	} else {
		fmt.Println("\nTenancy ID is set via OCI_CLI_TENANCY to: " + tenantId)
	}

	checkTenancy(tenantId, identityClient)

	compartmentInfo := getCompartmentInfo(tenantId, identityClient)

	if *flagListCompartments {
		fmt.Println("\nCompartments")

		for compartment := range compartmentInfo {
			println(compartment) // TODO: these need to be sorted
		}

		os.Exit(0)
	}

	// Anything past this point requires a compartment
	var compartmentName string
	compartmentIdEnv, exists := os.LookupEnv("OCI_COMPARTMENT_NAME")
	if exists {
		compartmentName = compartmentIdEnv
		fmt.Println("Compartment name is set via OCI_COMPARTMENT_NAME to: " + compartmentName)
	} else if *flagCompartmentName == "" {
		fmt.Println("Must pass compartment name with -c or set with environment variable OCI_COMPARTMENT_NAME")
		os.Exit(1)
	} else {
		compartmentName = *flagCompartmentName
	}

	compartmentId := compartmentInfo[compartmentName]
	if logLevel == "DEBUG" {
		fmt.Println("\n" + compartmentName + "'s compartment ID is " + compartmentId)
	}

	bastions := getBastionInfo(compartmentId, bastionClient)
	if *flagListBastions {
		fmt.Println("\nBastions in compartment " + compartmentName)
		for bastionName := range bastions {
			fmt.Println(bastionName)
		}

		os.Exit(0)
	}

	bastionId := bastions[*flagBastionName]
	getBastion(*flagBastionName, bastionId, bastionClient)

	if *flagListSessions {
		listActiveSessions(bastionClient, bastionId)
		os.Exit(0)
	}

	homeDir, err := os.UserHomeDir()
	checkError(err)

	var sshPrivateKeyFileLocation string
	if *flagSshPrivateKey == "" {
		sshPrivateKeyFileLocation = homeDir + "/.ssh/id_rsa"
		fmt.Println("Using default SSH private key file at " + sshPrivateKeyFileLocation)
	} else {
		sshPrivateKeyFileLocation = *flagSshPrivateKey
	}

	var sshPublicKeyFileLocation string
	if *flagSshPublicKey == "" {
		sshPublicKeyFileLocation = homeDir + "/.ssh/id_rsa.pub"
		fmt.Println("\nUsing default SSH public key file at " + sshPublicKeyFileLocation)
	} else {
		sshPublicKeyFileLocation = *flagSshPublicKey
	}

	publicKeyContent := getSshPubKeyContents(sshPublicKeyFileLocation)

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
			printSshCommands(bastionClient, sessionId, &sessionInfo.ip, &sessionInfo.user, &sessionInfo.port, sshPrivateKeyFileLocation)
		} else {
			fmt.Println("Session is no longer active. Current state is: " + sessionInfo.state)
		}

		os.Exit(0)
	}

	sessionInfo := checkSession(bastionClient, sessionId)

	for sessionInfo.state != "ACTIVE" {
		if sessionInfo.state == "DELETED" {
			fmt.Println("\nSession has been deleted, exiting")
			fmt.Println("State: " + sessionInfo.state)
			fmt.Println("\nSession Info")
			fmt.Println(sessionInfo)
			os.Exit(1)
		} else {
			fmt.Println("\nSession not yet active")
			fmt.Println("State: " + sessionInfo.state)
			fmt.Println("\nWaiting...")
			time.Sleep(10 * time.Second)
			sessionInfo = checkSession(bastionClient, sessionId)
		}
	}

	printSshCommands(bastionClient, sessionId, flagInstanceIp, flagSshUser, flagSshPort, sshPrivateKeyFileLocation)
}
