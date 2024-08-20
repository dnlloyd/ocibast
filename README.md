# ocibast
A tool for creating OCI bastion sessions and connecting to instances.

Note: 
- SSH connections are not yet supported in this version. Until SSH client support is added, SSH commands are generated and printed. See `Future enhancements` section below.
- The current version supports only the default OCI profile for auth

## Download

https://www.daniel-lloyd.net/ocibast/index.html

## Usage

### List compartments

Many commands require a compartment flag so this is useful for finding available compartments

```
ocibast -list-compartments
```

### List bastions

List available bastions for a compartment

```
ocibast -c compartment_name -list-bastions
```

### Create bastion session

Create a bastion session and print SSH commands

```
ocibast -c compartment_name -b bastion_name -i ip_address -o instance_id
```

example

```
ocibast -c mycompartment -b mybastion  -i 10.0.0.123 -o "ocid1.instance.oc1.iad.abcdefghitjlmnopqrstuvwxyz"
```
### List active bastion sessions

List active bastion sessions for a bastion

```
ocibast -c mycompartment -b mybastion -list-sessions
```

### Connect to an existing bastion session

Connect to an existing bastion session by session ID

```
ocibast -c compartment_name -b bastion_name -s session_ocd
```

example

```
ocibast -c mycompartment -b mybastion -s ocid1.bastionsession.oc1.iad.abcdefghitjlmnopqrstuvwxyz
```

### Help

```
ocibast -h
```

```
Environment variables:
The following environment variables will override their flag counterparts
   OCI_CLI_TENANCY
   OCI_COMPARTMENT_NAME

Defaults:
   SSH private key (-k): $HOME/.ssh/id_rsa
   SSH public key (-e): $HOME/.ssh/id_rsa.pub

Common command patterns:
List compartments
   ocibast -list-compartments

List bastions
   ocibast -c compartment_name -list-bastions

Create bastion session
   ocibast -b bastion_name -i ip_address -o instance_id

Create bastion session (long)
   ocibast -t tenant_id -c compartment_name -b bastion_name -i ip_address -o instance_id -k path_to_ssh_private_key -e path_to_ssh_public_key

List active sessions
   ocibast -c mycompartment -b mybastion -list-sessions

Connect to an active session
   ocibast -c compartment_name -b bastion_name -k path_to_ssh_private_key -e path_to_ssh_public_key -s session_ocd

Example of bastion session creation:
   export OCI_CLI_TENANCY=ocid1.tenancy.oc1..aaaaaaaaabcdefghijklmnopqrstuvwxyz
   export OCI_COMPARTMENT_NAME=mycompartment
   ocibast -b mybastion -i 10.0.0.123 -o ocid1.instance.oc1.iad.abcdefg

All flags for ocibast:
  -b string
    	bastion name
  -c string
    	compartment name
  -e string
    	path to SSH public key
  -i string
    	instance IP address of host to connect to
  -k string
    	path to SSH private key (identity file)
  -list-bastions
    	list bastions
  -list-compartments
    	list compartments
  -list-sessions
    	list sessions
  -o string
    	instance ID of host to connect to
  -p int
    	SSH port (default 22)
  -s string
    	Session ID to check for
  -t string
    	tenancy ID name
  -u string
    	SSH user (default "opc")
```

## Contribute

https://go.dev/doc/effective_go

```
go mod init github.com/dnlloyd/ocibast
```

```
go get -d github.com/oracle/oci-go-sdk/v65@latest
go mod tidy
```

### Build

#### Local OS/Arch

```
go build
```

#### OS/Arch specific

```
GOOS=darwin GOARCH=amd64 go build -o executables/mac/intel/ocibast
GOOS=darwin GOARCH=arm64 go build -o executables/mac/arm/ocibast
GOOS=windows GOARCH=amd64 go build -o executables/windows/intel/ocibast
GOOS=windows GOARCH=arm64 go build -o executables/windows/arm/ocibast
GOOS=linux GOARCH=amd64 go build -o executables/linux/intel/ocibast
GOOS=linux GOARCH=arm64 go build -o executables/linux/arm/ocibast
```

### Local install

```
go install
```

## Future enhancements and updates

- Add tests!
- Handle multiple OCI auth patterns
- Support more OCI environment variables
- Manage SSH client
  - https://pkg.go.dev/golang.org/x/crypto/ssh
- Manage SSH keys
  - https://pkg.go.dev/crypto#PrivateKey
- Implement basic instance search / list
