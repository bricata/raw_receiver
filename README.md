# raw_receiver
raw_receiver is a utility for receiving and storing Bricata's RAW JSON data export. While it was built specifically for working with Bricata exports, it can be used to receive and store any newline-delimited JSON stream transmitted over TCP.     

Specifically it does the following:
 
* Listens on a raw TCP socket; default = 9000
* Accepts multiple concurrent connections, from the same or multiple hosts
* Validates each JSON event before writing it to disk
* All events from a given host are written to a single file
* Monitors active stream files, rolls and compresses them once they reach the size limit; default = 1GB

## Installation

First clone the repo using git:  

<code>
	git clone https://github.com/bricata/raw_receiver.git
</code>

From here you can go one of two directions:

1) Build the binary using go

<code>
	cd raw_receiver
	go install
</code>>

This drops the raw_receiver binary in $GOPATH/bin.  You can then run it directly from there or copy it to your preferred location. 

2) If you don't want to build with go, you can use the pre-compiled binary included in the repo.  While this is not the best approach, it works on most \*nix platforms. As with option 1, copy the executable to the location you prefer to run it from.  

