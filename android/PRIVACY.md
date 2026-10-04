# Privacy policy: BombVault Android app

Last updated: 2 October 2026. Applies to version 9.7.0 and later, until this
date changes.

## The short version

The app opens the BombVault servers you run yourself and shows what runs on
them. It has no accounts, no advertising and no analytics of its own, and it
runs nothing in the background.

It talks to your servers directly, the way a browser does. Once it is paired
with your group, it also reaches them through a relay operated by the party
named under "Who is responsible". What the app and your servers say to each
other over the relay is encrypted with a key that only your own devices hold,
so the relay passes it along without being able to read it. The relay does see
that a phone is connected, from which IP address, and which server a message is
addressed to.

## What is stored on your phone

We back none of this up ourselves, and uninstalling the app deletes all of it.

- Your server list: for each server the name you gave it, its address, the
  SHA-256 fingerprint of a certificate you chose to trust, and, for a server
  from your group, its instance ID.
- Your group's key, derived from the twelve words. It is sealed with a key that
  never leaves the Android keystore. The words themselves are not stored.
- A random device ID such as `9f2c41d07a6e85b3c0d4e1f2a7b8c9d0`. It contains
  nothing about you or your phone. The group uses it to recognise this phone.
- The name you give the phone in Settings, if you give one.
- What the app's web view keeps for each server, as a browser would: the session
  cookie once you sign in, and the interface settings the server's own pages
  store.
- Your settings: the interface language, the look (theme, corner shape, accent
  colour, rainbow mode, and whether to follow the first server's look), and the
  motion level.

If you use Android's own backup, it may include the app's storage. The group
key is sealed with a key that stays in this phone's keystore and is not part of
any backup, so a restored copy cannot read it.

## What leaves your phone

### To your own servers

The app opens each server at the address you saved, over HTTPS or HTTP as the
address says, and sends its session cookie the way a browser does. While the
start screen is open it asks every server what runs there, and while the app
follows the first server's look it asks that server for its look.

### On your local network

While the start screen is open, the app listens for BombVault servers that
announce themselves on the local network (DNS-SD). It announces nothing itself.

### To the relay

The app connects to the relay only while it is paired with a group. The relay
is `relay.halleluja.design`, on a server in Germany. Every connection to it
carries:

- A group key derived from your phrase with a one-way hash. It cannot be turned
  back into the words. Whoever presents this key joins your group, so it works
  like a password for the group.
- Your random device ID, and the ID of the server a message is for, which the
  relay needs for routing.
- Encrypted messages (AES-256-GCM, with a second key derived from your phrase
  that the relay never receives). The relay cannot read their content, which
  includes the phone's name, what runs on your servers and their look.
- Your IP address, as with any connection on the internet.

The relay keeps no record of who connects or what they send, and its error log
never contains IP addresses. The one exception is a connection that fails the
relay's own handshake, for example one that closes or stalls before it
identifies itself: its IP address is held in memory to slow down repeated
failures, and deleted within 61 minutes of that address's last failed attempt.

### When you scan a QR code

The QR code beside your phrase can be scanned instead of typing the words. The
app reads the camera picture on the phone with the free jsQR library, and
neither stores nor sends it. No one else takes part.

### When you open a link or donate

- The GitHub, PayPal, mail, Unraid, Docker and source code buttons, and the
  version numbers, open your browser or mail app. The app itself sends nothing
  there.
- The coffee button opens Buy Me a Coffee's own page inside the app, which then
  receives what a browser would send it, under its own privacy policy.
- The crypto window shows donation addresses and draws their QR codes on the
  phone.

The "Copy report" button in Settings puts the app version, the Android API
level, the language and the look on the clipboard, for you to paste into a bug
report. Nothing is sent.

The app makes no other network requests.

## Permissions

| Permission | Used for |
| --- | --- |
| Camera | Scanning the QR code of your phrase. Asked for when you open the scanner and press "Allow access". |
| Internet | Reaching your servers and the relay. |

The app asks for no notification, location, contacts, storage or microphone
permission. Files you export from a server are saved to Downloads through
Android's own storage, which needs none. It shows no notifications and does no
work while it is closed.

## What the app does not do

- It has no analytics, telemetry or crash reporting of its own.
- It shows no advertising, and we do not sell data or share it with anyone.
- It backs nothing up itself. Backups run on your servers.

## Keeping your data safe and deleting it

- Everything between the app and your servers over the relay is encrypted as
  described above, and the connection to the relay uses TLS.
- **Remove a server** with the pencil on its card, or all of them in Settings,
  which also leaves the group.
- **Uninstall the app** to delete everything it stored.
- **Relay data:** while a connection is open, the relay holds what it needs to
  route it (your IP address, the group key, your device ID and the encrypted
  messages passing through) and drops all of it when the connection closes.
  Beyond that it holds nothing about you except the rate-limit entry described
  above. For any question about it, or to exercise your rights, write to the
  contact address below.

The phrase belongs to your whole group. Leaving the group on one phone does not
change the phrase on your servers; to lock a lost phone out, remove its card on
the Instances page and pair your servers again with new words.

## Who is responsible

The relay and this policy are the responsibility of:

Georg Düringer (Halleluja Design)
privacy@halleluja.design

The relay runs on a server rented from Hetzner Online GmbH, Germany, which
processes this data on our behalf.

Under the EU General Data Protection Regulation, forwarding your messages rests
on Art. 6(1)(b), because it is the service you use the app for, and the
short-lived rate-limit entry rests on Art. 6(1)(f), our interest in keeping the
relay available. You have the right to access, correct and delete your data, to
restrict or object to its processing, to receive it in a portable format, and to
complain to a data protection supervisory authority.

## Children

The app is a tool for operating your own server software and is not directed at
children.

## Changes

When this policy changes, the date at the top changes with it, and every change
is visible in the repository's history.

## Source

The app is free software under the AGPL-3.0. Everything described here can be
checked against the source:
https://github.com/junkerderprovinz/bombvault/tree/main/android
