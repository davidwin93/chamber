# Chamber

The goal of this project is to provide on demand Firecracker VMs. 

## How

Listen on a TCP/UDP port and when a connection comes in connect to an existing VM or if one is not avaliable create it from an existing snapshot or as a new VM. 
For simplicity we bundle a simple alpine OS with containerd and bootstrap docker images into this image to allow you to run any service currently. In the longer term we'd like to convert docker contaienrs into native firecracker VMS.

## ToDo

* Add documentation around configuring Firecracker and building a base VM image
* Refactor to add more configuration (VM resources, IP ranges, etc..)
* Snapshot and hibernate VMs when they have been idling for some time.
* Run VMs in Jailer


## Support Docker Container natively 
Read manifest.json -> grab config json -> grab config.cmd -> run as init in custom init script/golang runner.