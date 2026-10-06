# Android app

The Android app puts every BombVault server of your group on your phone. It starts on a list of your servers with the activity log of all of them on top, the same lines the Dashboard shows, and opens the phone view of whichever server you tap. The app backs nothing up itself.

## Getting the app {#install}

- **Settings, Apps:** the Android app's card offers the APK for the release your server runs, with a QR code to scan from the phone.
- **APK:** every release has `bombvault-android.apk` on its release page, and [this link](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) always downloads the latest build. It needs Android 10 or later, and Android asks once whether the app you open the file with may install apps.
- **Google Play:** the app is in a closed test until it can go public. Google Play lists an app from a new developer account only after at least 12 testers have kept it installed for 14 days. To help, join the [tester group](https://groups.google.com/g/arrowloop-testers), open the [test page](https://play.google.com/apps/testing/bombvault.halleluja.design), tap **Become a tester** and install BombVault from Google Play.
- **F-Droid:** the listing follows.

Pair the app with servers on version 9.7.0 or later. A server on an older version can be in the same group, but it shows the phone as a plain instance, and the app reads that server's activity only after a sign-in.

## Pairing by QR code {#pairing}

1. On any server of your group, open **Settings, Pairing** and choose **Show phrase**. The twelve words appear with a QR code beside them.
2. In the app, tap **Scan QR code** and point the phone at the code. You can also paste or type the words.
3. The app lists the servers of that group before it saves anything. **Add all** adds every one of them.

The phone then joins the group like another instance. It reads what runs on each server through the group without a sign-in, at home directly and away from home over the relay. How the group itself works is described under [Pairing instances](offsite-recovery.md#pairing).

!!! note "The interface still needs a way to the server"
    The server list and the activity log come through the group. The interface of a server opens directly, so the phone has to reach the server's address, at home or over a VPN.

## Signed in on a paired phone {#sign-in}

A phone paired with your group opens each of its servers already signed in. Before it loads a page, it asks that server over the group for a session, and a server hands one only to a member that is a phone. A server added by its address asks for the password, as in a browser. Anyone with the twelve words can already open every backup of the group, so pairing grants nothing new.

## Servers outside a group {#other-servers}

- **Add server** takes the address you open BombVault with in a browser, such as `192.168.1.10:3443`. Without `http://` or `https://` in front, the app uses https.
- Servers that announce themselves on the local network are listed under **On this network** and open with one tap. They do this while **Find on the network** is switched on under Settings, Integrations. A server in another network or behind a VPN does not show up there.
- A self-signed certificate is trusted once by its SHA-256 fingerprint. When the server later shows a different one, the app warns you and opens it only after you trust the new certificate.

## The phone on the Instances page {#instances}

The phone gets its own card on the Instances page of every server in the group, marked as an Android app and named after the name you gave the phone. It has no scorecard because it backs nothing up, and **Remove** takes it off the page.

## Settings {#settings}

The gear beside the plus opens the app's settings:

- the language, and the name the phone shows on the Instances page (empty means the phone's model),
- the look, which follows the first server in the list until you set your own, and animations, which have a setting of their own,
- a report to copy when you report a problem; it contains no address, name or phrase,
- the About card with the privacy policy,
- **Remove all servers**, which removes every server from the app and leaves the group. Nothing changes on the servers themselves.

## Downloads and uploads {#files}

Exports, recovery kits, flash ZIPs and database dumps land in the phone's Downloads folder, as they would from a browser. A settings import opens the phone's file picker.

## On a touch screen {#touch}

Nothing hovers under a finger, so a control dims while it is held, and a button with a brand logo lights up in its brand colour until the finger lifts. A long press on a button counts as a slow tap and opens no link menu.

## In a browser instead {#browser}

Chrome and Edge can install BombVault's web interface as an app in its own window, on a phone as on a computer. Nothing is cached, so an update shows at once.

## Privacy {#privacy}

The app has no accounts, no advertising and no analytics, and it runs nothing in the background. Its [privacy policy](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) lists what it stores and what it sends where.
