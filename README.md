# raw_receiver
raw_receiver is a utility for receiving and storing Bricata's RAW JSON data export.    

Specifically it does the following:
 
* Listens on a raw TCP socket; default = 9000
* Accepts multiple concurrent connections, from the same or multiple hosts
* Validates each JSON event before writing it to disk
* All events for a given host (CMC or sensor) are written to a single file
* Monitors active stream files, rolls and compresses them once they reach the size limit; default = 1GB (e.g. 1000000000 bytes) 

