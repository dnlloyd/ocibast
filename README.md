# oci-bastion-client
A tool for creating OCI bastion sessions and connecting to instances.

Note: SSH connections are not yet supported in this version. Until SSH client support is added, SSH commands are generated and printed. See `Future enhancements` section below.

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

Create a bastion session and print SSH command

```
ocibast -c compartment_name -b bastion_name -i ip_address -o instance_id -k ssh_private_key -e ssh_public_key
```

example

```
ocibast -c mycompartment -b mybastion  -i 10.0.0.123 -o "ocid1.instance.oc1.iad.abcdefghitjlmnopqrstuvwxyz" -k $HOME/.ssh/oci/id_rsa -e $HOME/.ssh/oci/id_rsa.pub
```
### List active bastion sessions

List active bastion sessions for a bastion

```
ocibast -c mycompartment -b mybastion -list-sessions
```

### Connect to an existing bastion session

Connect to an existing bastion session by session ID

```
ocibast -c compartment_name -b bastion_name -k ssh_private_key -e ssh_public_key -s session_ocd
```

example

```
ocibast -c mycompartment -b mybastion -k $HOME/.ssh/oci/id_rsa -e $HOME/.ssh/oci/id_rsa.pub -s ocid1.bastionsession.oc1.iad.abcdefghitjlmnopqrstuvwxyz
```

### Help

```
ocibast -h
```

```
Usage of ocibast:
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
  -u string
    	SSH user (default "cloud-user)
```

## Contribute

https://go.dev/doc/effective_go

```
go mod init local/ocibast
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
GOOS=darwin GOARCH=amd64 go build -o hello-darwin-amd64
```

### Local install

```
go install
```

## Future enhancements and updates

- Add tests!
- Manage SSH client
  - https://pkg.go.dev/golang.org/x/crypto/ssh
- Manage SSH keys
  - https://pkg.go.dev/crypto#PrivateKey
- Support SSH tunneling
- Support SCP
- Implement basic instance search / list
