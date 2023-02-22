# ProtoGen

ProtoGen (**Gen**eric **Proto**col) is a collection of useful and educational application-layer protocols written in Go.

Currently:
* ProtoQute/TCP/Random Quote Protocol
* ProtoMath/TCP/Equation Solve Protocol
* ProtoDir/UDM/Directory Listing Protocol

* [ProtoDir](#protodir)
* [ProtoQuote](#protoquote)

# ProtoDir

ProtoDir is a directory lister, with the ability to:

* List files and subdirs separately, and together.
* Walk the directory tree
* Read file
* Stat files and dirs

ProtoDir uses Unix Domain Sockets to communicate. It is easy to run ProtoDir.

```
protogen [proto]dir --path[-p] <path to socket file> [--clear_interval[-c] interval to clear states] [--ttl[-t] time for states to live]
```

for example:

```
protogen dir -p /tmp/protodir.sock -t 20 
```

or 

```
protogen protodir -p /tmp/protodir_new.sock
```

When you quit the program with SIGTERM using Ctrl + C the socket file will be deleted. Otherwise there is no guarantee that the socket file will remain there or not. 

After spawning a listener you can use Netcat to communicate with the socket.

```
echo 'PTDP v1 INIT_STATE <path>' | nc -U /tmp/protodir.sock
```

The syntax for requests are:

```
PTDP v1 <Command> [path or hashes]
```

After you had sent an `INIT_STATE` command it will return you the hash of your state like so:

```
12 - INIT_OK

2672c342d
```

You will need to CD now. To CD you need to send two hashes. The first hash being the hash of the state and the second hash being the hash of the subdirectory. **But you need to CD to root dir first** and for that you need to use the state hash as dir hash.

```
PTDP v1 CD_SUBDIR 2672c342d;2672c342d
```

where you will see:

```
13 - CD_OK




```

And then you can list directories and files:

```
PTDP v1 LIST_DIR 2672c342d
```

Which you will get:

```
32 - DIR_LISTED

$LIST_DIR: /home/chubak-eniac/aa;

*f*path=a_file.txt*hash=6ca080b6
===

+d+path=a_subfolder+hash=6777e8ae


```

and then you can stat the file or folder:

```
PTDP v1 STAT_ENTITY 2672c342d;6777e8ae
```

which you will see

```
23 - STAT_OK

$STAT_ENTITY: /home/chubak-eniac/aa/a_subfolder;
IsDir: true;
ModTime: 2023-02-22 13:47:11.718434331 +0330 +0330;
Mode: drwxrwxr-x;
Name: a_subfolder;
Size: 4096;
	

```

There's 2 newlines between message and body, and 2 newlines between body and end of the message. There's one new line between header (like `$STAT_ENTITY`) and messages start with one newline save for the readbytese message because it will mess up the file.

And finally to view all the states:

```
PTDP v1 LIST_STATES
```

The final list of all commands:

* INIT_STATE (path)
* CD_SUBDIR (2 hash)
* LIST_DIR (1 hash)
* LIST_FILES (1 hash)
* LIST_SUBDIRS (1 hash)
* READ_BYTES (2 hash)
* STAT_ENTITY (2 hash)
* WALK_TREE (1 hash)
* LIST_STATES (no hash)

# ProtoMath

Run it:
```
protogen math -a <tcp addr with port>
```

And then `echo 'PTMP/1 2 + 2' | nc <addr> <port>`

# ProtoQuote

ProtoQuote is a simple random quote generator. Start it with

```
protogen quote --address[-a] <TCP address with port> [--interval[-i] quote generation interval]
```

Then use netcat to connect to that address and port to see a quote.