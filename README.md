# Chamber

The goal of this project is to provide on demand Firecracker VMs backed by OCI container images.  

## How

Listen on a TCP/UDP port and when a connection comes in connect to an existing VM or if one is not available create it from an existing snapshot or as a new VM. 
When defining a new VM a docker container is converted into a firecracker VM that can then be reused to deploy new VMs as needed.

## ToDo

* Add documentation around configuring Firecracker and building a base VM image
* Refactor to add more configuration (VM resources, IP ranges, etc..)
 * this is partially done and the config is extracted
* Snapshot and hibernate VMs when they have been idling for some time.
* Run VMs in Jailer
* Add support for custom kernels 


## Support Docker Container natively 
Read manifest.json -> grab config json -> grab config.cmd -> run as init in custom init script/golang runner.
Still needs to handle work dir and other custom configuration to support all docker containers 
