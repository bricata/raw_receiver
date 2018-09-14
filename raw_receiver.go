package main

import (
    "fmt"
    "crypto/md5"
    "path/filepath"
    "strings"
    "io/ioutil"
    "log"
    "io"
    "bufio"
    "time"
    "os"
    "flag"
    "net"
    "encoding/json"
    "strconv"
    "compress/gzip"
    // "html"
)

// #  Design priniciples:
// #   listen on raw TCP socket
// #   accept incoming connections
// #   run dedicated receiver thread for each client
// #   receiver spawns itself a writer thread
// #   split messages on newline
// #   parse, validate and write JSON to 'activeDir' queue file
// #   file manager process, rolls and compresses files hourly
// #        fmer reads and filters JSON's before archiving
// #        organized by timestamp and sensor 
// #        files compressed and stored in designated location
// #        seperate file manager goroutine enforces file size limits and 
// #        handles archiving

// Declare variables set by the cli that need to be accessible in 
// multiple functions
var activeClients = make(map[string]client)
var maxSize int64
var maxAge  float64
var activeDir  string
var archiveDir string

type client struct {
    // incoming chan string
    events    chan string
    conns     map[net.Conn]bool
    //cid     string
    outFile   string
    addr      string
}

func genToken() (string) {
    // Generate a unique token based on the current time
    crutime := time.Now().Unix()
    h := md5.New()
    io.WriteString(h, strconv.FormatInt(crutime, 10))
    token := fmt.Sprintf("%x", h.Sum(nil))
    return token
}

//func manageStreams(t <-chan time.Time, archiveDir string, activeDir string)() {
    // Perform routine checks on dir contents and file inventory
//    for now := range t {
       // check the size of all files in the activeDir dir
//    }
//}

// Forget and close the connection
func dropConn(c net.Conn, cip string) {
    delete(activeClients[cip].conns, c)
    c.Close()
}

// Test if string is JSON
func isJSON(s string) bool {
    var js map[string]interface{}
    return json.Unmarshal([]byte(s), &js) == nil

}

func compress(source string) {
    // compress the specified file in place 

    // Open the source file
    f, err := os.Open(source)
    if err != nil {
        log.Printf("Unable to open file %s for compression: %s", source, err)
        return
    }

    defer f.Close()

    // open a file handle for gzipped archiveDatePath 
    archName := source + ".zip"
    out, readErr := os.OpenFile(archName, os.O_WRONLY|os.O_CREATE, 0666)
    if readErr != nil {
        log.Printf("Unable to create zip archive %s: %s", archName, readErr)
        return
    }
    defer out.Close()

    writer := gzip.NewWriter(out)

    // copy the reader to the writer 
    _, writeErr := io.Copy(writer, reader)
    if writeErr != nil {
        log.Printf("Data copy failed, unable to compress: %s", writeErr)
        return
    }

    f.Close()
    os.Remove(source)
}

func archiveFile(file string, info os.FileInfo) {
    // Move files from activeDir to archiveDir
    // We use a sub-directory structure that makes finding files 
    // by date easier:  YYYY/MM/DD
    // 
    // Determine which directory the file goes in by the last
    // modified time.  
    // 
    // Check to make sure directories exist, if not create it
    year, month, day := info.ModTime().Date()
    hour, _, _ := info.ModTime().Clock()

    archiveDatePath := filepath.Join(archiveDir, strconv.Itoa(year), strconv.Itoa(int(month)), strconv.Itoa(day))
    if _, err := os.Stat(archiveDatePath); os.IsNotExist(err) {
        // the directory does not exist, try to create the entire path
        err := os.MkdirAll(archiveDatePath, 0700)
        if err != nil {
            // Unable to create the archive directory, log the error and exit the program
            log.Printf("Error: %s creating archive directories.", err)
            return 
        }
    }

    // Get YYDDMMHH from the file mod time
    dtString := fmt.Sprintf("%v%v%v%v", year, int(month), day, hour)

    newFile := filepath.Join(archiveDatePath, strings.TrimSuffix(filepath.Base(file), "active") + dtString )
    // move the file so it can't be written to
    err := os.Rename(file, newFile)
    if err !=  nil {
        log.Printf("Error %s renaming file %s to %s", err, file, newFile)
        return
    }
    
    go compress(newFile)

    // See if there is a client using this file
    // if so restart the connection
    for _, client := range activeClients {
        if client.outFile == file {
            // found one, close related connections
            for c, _ := range client.conns {
                c.Close()
            }
        }
    }  
}

// Function to roll an active file in place, instead 
// of using the archive directory tree
func rollFile(file string, info os.FileInfo) {
    // Gather data and time info from file mod time
    year, month, day := info.ModTime().Date()
    hour, min, _ := info.ModTime().Clock()

    dtString := fmt.Sprintf("%v%v%v%v%v", year, int(month), day, hour, min)

    if strings.HasSuffix(file, "active") {
        newFile := filepath.Join(filepath.Dir(file), strings.TrimSuffix(filepath.Base(file), "active") + dtString)
        // move the file so it won't be written to anymore
        err := os.Rename(file, newFile)
        if err !=  nil {
            log.Printf("Error renaming file: %s", err)
            return
        }
        go compress(newFile)    
    } 
    else {
        go compress(file)
    }

    // See if there is a client using this file
    // if so restart the connection
    for _, client := range activeClients {
        if client.outFile == file {
            // found one, close related connections
            for c, _ := range client.conns {
                c.Close()
            }
        }
    }
}

func manageLogs(t <-chan time.Time) {
    // On every interval, gather contents of the activeDir directory
    // then check each for size or age threshold violation. 
    // If we find a file in violation, delete file if its empty, archive all 
    // others.  
    for now := range t {
        // Evaluate files in activeDir
        activeFiles, err := ioutil.ReadDir(activeDir)
    
        if err != nil {
            log.Printf("Error %s reading directory %s", err, activeDir)
            continue
        }

        for _, f := range activeFiles {
            // Ignore directories
            if f.IsDir() == true { continue }

            fpath := filepath.Join(activeDir, f.Name()) 
            staleMins := now.Sub(f.ModTime()).Minutes()

            if staleMins > maxAge || f.Size() > maxSize {
                // Delete empty files 
                if f.Size() == 0 {
                    os.Remove(fpath)
                    continue
                }
                // Otherwise archive it
                //go archiveFile(fpath, f)
                go rollFile(fpath, f)
            }
        } 
    }
}

//func getWriter(path string) (writer *bufio.Writer) {
//    // Create an output writer.   
//    outFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0666)
//    if err != nil {
//        // Can't create the output file, exit
//        log.Printf("Error %s. Unable to create file for active stream. Exiting.", err)
//        return
//    }
//
//    w := bufio.NewWriter(outFile)
//    
//    return w
//}

func logStream(c *client) {
    // Receive events on the clients channel and write them to the file
    //  TODO{ Add other output formats. }

    outFile, err := os.OpenFile(c.outFile, os.O_WRONLY|os.O_CREATE, 0666)
    if err != nil {
        // Can't create the output file, exit
        log.Printf("Error %s. Unable to create file for active stream. Exiting.", err)
        return
    }

    //w := bufio.NewWriter(outFile)

    //w := getWriter(c.outFile)
    
    // Receive events on the channel and write them to the ouput file
    for evt := range c.events {
        bytes, err := outFile.WriteString(evt + "\n")
        if err != nil {
            log.Printf("Error: %s.  %s bytes written.", err, bytes)
            return
        }
    }
    //w.Flush()    
}

// Manage new connections to the receiver
func handleConnection(c net.Conn) {
    // Create a token for this conn based on the current absolute time
    // cid := genToken()

    // Grab the client IP
    clientIP := strings.Split(c.RemoteAddr().String(), ":")[0]
    clientPort := strings.Split(c.RemoteAddr().String(), ":")[1]

    // Log the the new session
    log.Printf("Connection established with %s on %s", clientIP, clientPort)

    // Create the file path
    fname := filepath.Join(activeDir, clientIP + ".active") 

    // Check if we've seen this IP already
    if _, ok := activeClients[clientIP]; ok == false {

        client := &client{
            events:   make(chan string, 2),
            conns:    map[net.Conn]bool{ c : true},
            addr:     clientIP,
            outFile:  fname,
        }

        activeClients[clientIP] = *client

        go logStream(client)

    } else {
        activeClients[clientIP].conns[c] = true
    }

    rdr := bufio.NewReader(c)

    for {
        input, err := rdr.ReadString('\n')

        if err == io.EOF {
            log.Printf("Received EOF. Terminating connection %s with host %s.", clientPort, clientIP)
            break
        } else if err != nil {
            log.Printf("Connection error %s. Terminating connection %s with host %s.", err, clientPort, clientIP)
            break
        }

        if r := isJSON(input); r != true {
            log.Printf("JSON parse error: %s for value:  %s", err, input)
            continue
        }

        // write JSON string to writer channel
        activeClients[clientIP].events <- strings.TrimSuffix(input, "\n")
    }
    
    dropConn(c, clientIP)
}

func main() {

    flag.Int64Var(&maxSize, "max_size", 1000000, "Max size in bytes an active file can reach before being archived.")
    flag.Float64Var(&maxAge, "max_age", 60, "Number of minutes a file can be inactive before being archived.")

    flag.StringVar(&archiveDir, "archive_dir", "/var/log/bricata/archive", "Directory for archiving Raw Bricata export files.")
    flag.StringVar(&activeDir, "active_dir", "/var/log/bricata/active", "Directory for handling active Raw Bricata export streams.")

    // cert_pem := flag.String("cert", "cert.pem", "Server TLS certificate.")
    // key_pem := flag.String("key", "key.pem", "Server TLS certificate key.")
    receiverPort := flag.String("port", "9000", "Local TCP port for receiving Raw Bricata exports.")
    receiverIP := flag.String("ipaddress", "0.0.0.0", "Local IP for receiving Raw Bricata exports.")
    
    flag.Parse()

    // Create a 30 second time ticker
    t := time.Tick(time.Minute / 2)
    // Start the log manager goroutine
    go manageLogs(t)

    PORT := *receiverIP + ":" + *receiverPort
    l, err := net.Listen("tcp4", PORT)
    if err != nil {
        log.Printf("%s", err)
        return
    }
    log.Printf("Receiving JSON stream on port %s", *receiverPort)

    defer l.Close()

    // Spawn handler goroutine for each new connection.
    // Typically the same socket will be re-used, but  
    // each sensor may connect more than once.
    for {
        c, err := l.Accept()
        if err != nil {
            log.Printf("%s", &err)
            return
        }
        go handleConnection(c)
    }

}
