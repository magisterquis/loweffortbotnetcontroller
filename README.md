Low Effort Botnet Controller
============================
The low effort parts refers to writing it, not using it.  Also refers to
documenting it.

Features
--------
- Notes when bots check in.
- Can task bots to call back.
- Meant to work with curlrevshell.
- Increased appreciation for documentation.

Quickstart
----------
Also nearly the entirety of the useful parts of the documentation at the moment.

1.  Have the Go Compiler installed.
2.  Probably be using OpenBSD, though it may work with bmake on Linux.
3.  Edit the top of [`bot.sh.m4`](./bot.sh.m4).
4.  Be in this repo and `make`.
5.  `./loweffortbotnetcontroller` and fiddle with flags (try
    `./loweffortbotnetcontroller -h`) until it works.  On Linux, probably need
    `sudo setcap cap_net_bind_service+ep ./loweffortbotnetcontroller`.
6.  Start with `./loweffortbotnetcontroller $YOUR_FLAGS >/dev/null 2>&1 &`.
7.  For catching callbacks: `go install
    github.com/magisterquis/curlrevshell@latest`, then run it and copy/paste
    one of the "To get a shell:" lines to
    `$HOME/loweffortbotnetcontroller.d/callback.sh`.
8.  Spray `bot.sh` out to targets.
9.  Have a look in `$HOME/loweffortbotnetcontroller.d/checkins` for a list of
    bots which have checked in.  Each file's name is a bot ID.  The contents are
    (hopefully) a process listing.
10. To get a callback: Have curlrevshell ready and `touch
    $HOME/loweffortbotnetcontroller.d/callbackrequests/BOT_ID`.  Watch
    `$HOME/loweffortbotnetcontroller.d/log.json`.
11. Wonder why on earth you're using this thing.

Usage
-----
```
Usage: loweffortbotnetcontroller [options]

A botnet controller written with very low effort.

Options:
  -callback-string string
    	String sent in reply to a checkin to request a callback (default "loweffortbotnetcontroller_callback")
  -debug
    	Enable debug logging
  -dir string
    	LowEffortBotnetController's directory (default "/home/you/loweffortbotnetcontroller.d")
  -listen address
    	HTTPS listen address (default "0.0.0.0:443")
  -max-callback int
    	Maximum callback body to keep (default 1048576)
  -max-ids number
    	Maximum number of IDs to track (default 100000)
  -print-callback-string
    	Print the callback string and exit
  -print-fingerprint
    	Print the TLS fingerprint and exit
  -tls-certificate-cache file
    	Optional file in which to cache generated TLS certificate (default "/home/you/.cache/sstls/cert.txtar")
  -update-every interval
    	ID Tracking and Callback Request update interval (default 1s)
```

Theory
------
...or parts of it, at any rate.

1.  All things happen in a directory, probably
    `$HOME/loweffortbotnetcontroller.d`.
2.  The program spawns an HTTPS server with a self-signed cert.
3.  Bots check into the server with a request for `/checkin/{id}`.
4.  The body of the checkin request is saved to `checkins/{id}`.
5.  The response will be a specific string if a callback has been requested.
6.  Callbacks are requested by making a file in `callbackrequests/`, which
    will the server will see and use to update the itss internel state, which
    will in turn be written to `id.json`.  The file in `callbackrequests/` will
    be deleted.
