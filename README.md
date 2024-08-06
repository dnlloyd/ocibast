# oci-bastion-client
A tool for connecting to OCI bastions

## Usage
```
Usage of ocibast:
  -b string
    	Name of bastion
  -c string
    	Name of compartment
  -i string
    	Instance IP address of host to connect to
  -list-bastions
    	List bastions
  -list-compartments
    	List compartments
  -o string
    	Instance ID of host to connect to
```

## Contribute

```
go mod init local/ocibast
```

```
go get -d github.com/oracle/oci-go-sdk/v65@latest
go mod tidy
```
