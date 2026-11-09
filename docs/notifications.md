# Notifications

BombVault can tell you when a backup fails, when it succeeds, when one is overdue, and once a week how things went. Everything is set under **Settings, Notifications**: what BombVault sends beyond single backups in the **Notifications** card, and where messages go in the **Channels** card. Each channel decides for itself when it speaks up, and a Healthchecks check is a channel too.

![Settings, Notifications with the Gotify channel filled in.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Settings, Notifications with the Gotify channel filled in.](assets/screenshots/notifications/gotify-light.png#only-light)

## What sends a message {#what}

The switches in the **Notifications** card apply to every channel:

- **Summarise scheduled runs**: one message per scheduled run, such as *42 of 45 succeeded*, instead of one per container or VM. A backup you start by hand still sends its own message. A frozen guest, a failed retention or a crossed growth budget goes out right away even with summaries on.
- **Notify on container update**: one message for each container that **Update containers after a successful backup** moved to a newer image, so you can check that it still works.
- **Report overdue backups**: once a day at 09:00 BombVault checks whether a switched-on backup is overdue, which means older than twice its schedule, and sends one message per case, naming the entry that fell behind. The next successful backup clears it.
- **Weekly digest**: one summary a week with run counts, new backup data, which sections are overdue, whether the off-site copies are current and the top failures. Its plan is a button like every other plan, and **Off** switches it off. See [Schedules](schedules.md#plans).

Whether a channel hears about successes too is set on the channel, under **Notify**. Which anomalies send a message is set under **Settings, Verify**, see [Anomalies](anomalies.md). A deferred backup sends one message when it starts to wait and one when the wait ends, not one per try.

## Channels {#channels}

**Add channel** at the foot of the **Channels** card offers **Unraid notifications**, **ntfy**, **Gotify**, **Discord**, **Slack**, **Matrix**, **Email (SMTP)**, **Apprise**, **Webhook** and **Healthchecks**. Add as many as you like, two of the same kind as well, such as one ntfy topic for failures and another for everything. A new channel starts switched off, so it is saved before its fields are filled in.

Every channel has:

- its own switch, which turns it off without losing its fields.
- **Notify**: **Never**, **Only on failure** or **On success and failure**. A new channel starts on **Only on failure**. A backup that succeeded but left something out reaches a failure-only channel as well.
- **Name**, which is optional and stands in place of the kind, handy when you have two of a kind.
- **Send test**, which sends a test message through this channel with the values as they stand in the form, saved or not. The line under the channel says whether the service took it, and why not.
- **Remove channel**, the bin next to it.

Each channel sends on its own with its own time limit, so one host that never answers does not hold up the others. When a service fails in a way another try could fix, the channel tries up to three times. An overdue, budget or anomaly alert that no channel took is tried again later, and so is a weekly digest.

| Channel | Fields | Notes |
|---|---|---|
| **Unraid notifications** | none | Sends to Unraid's own notification system, which can forward to Pushover, email or Discord. It runs over the SSH connection from **Settings, Integrations, Host SSH**, so that key has to be authorised. On TrueNAS and other hosts it stays silent, and the channel says why. |
| **ntfy** | **Topic URL**, **Access token (optional)** | The address ends in your topic. A token is only needed for a protected topic. |
| **Gotify** | **Server URL**, **App token**, **Priority on failure**, **Priority on success** | Priorities run from 0 to 10 and start at 8 for failures and 5 for successes. |
| **Discord** | **Webhook URL** | The address of a webhook from the Discord channel's settings. |
| **Slack** | **Webhook URL** | The address of an incoming webhook of your Slack app. |
| **Matrix** | **Homeserver URL**, **Access token**, **Room ID** | The account behind the token has to be in the room. |
| **Email (SMTP)** | **SMTP host**, **Port**, **Encryption**, **Username**, **Password**, **From address**, **To address** | **Encryption** is **STARTTLS**, **TLS (implicit)** or **None**. |
| **Apprise** | **Apprise API URL**, **Tags (optional)**, **Apprise addresses (optional)** | Point it at the `/notify/<key>` endpoint of your Apprise API container to reach its 100+ services, and tags pick a subset of that key's targets. Or list Apprise addresses, one per line, such as `tgram://` for Telegram or `pover://` for Pushover, and end the API URL in `/notify` without a key. |
| **Webhook** | **Webhook URL**, **Method**, **Headers**, **Body** | A request you shape yourself, see [Webhook](#webhook). |
| **Healthchecks** | **Ping URL** | See [Healthchecks](#healthchecks). |

Secrets such as the tokens, the SMTP password and the values of webhook headers are stored encrypted, and the server never hands them back to the page. Under a saved secret the field says **Saved. Leave the field empty to keep it.** When you change where a channel sends to, BombVault does not send the old secret to the new place: the field says **The new address is saved once you enter this again.**, and the new address is saved once you type it.

Settings from a version before the channel list are carried over at the first start, and nothing is lost. Each channel you had filled in becomes an entry of its own with the **Notify** choice you had. A webhook set to the Discord, Slack, Gotify or ntfy format becomes a channel of that kind, and a Generic JSON webhook keeps posting `title`, `message` and `ok` through its **Body**. The Unraid switch becomes an Unraid channel, and the Healthchecks URL with its exceptions a Healthchecks channel.

### Webhook {#webhook}

A **Webhook** channel sends one HTTP request to any address. **Method** is GET, POST or PUT. **Add header** adds a row under **Headers**, and header values are kept as secrets, so an `Authorization` header is safe there. **Body** is sent just as you write it, with these placeholders filled in:

| Placeholder | Becomes |
|---|---|
| `%%title%%` | the title of the message |
| `%%body%%` | the message itself |
| `%%ok%%` | `true`, or `false` for a failure |
| `%%event%%` | what happened, such as `container.failed` |
| `%%severity%%` | `info`, `success`, `warning`, `failure` or `critical` |
| `%%importance%%` | Unraid's level for it: `normal`, `warning` or `alert` |
| `%%link%%` | a link that belongs to the message, empty when there is none |
| `%%key%%` | the section, such as `container` or `VM` |
| `%%time%%` | the time in UTC, such as `2026-11-09T03:00:00Z` |
| `%%source%%` | `BombVault` |

Placeholders work in the address and in header values too, and each value is escaped for where it stands: as JSON when the `Content-Type` header says JSON, and as a query value in the address. A new webhook starts with that header and posts `{"title":"%%title%%","message":"%%body%%","ok":%%ok%%}`. A name BombVault does not know stays as written, so a typo shows in the message. Leave **Body** empty to send none.

## Healthchecks {#healthchecks}

A service such as [Healthchecks.io](https://healthchecks.io) raises the alarm when BombVault stops checking in. Add a **Healthchecks** channel and paste the check's **Ping URL**, and BombVault pings it for the whole lifecycle of every backup: at the start, then on success or with `/fail`. This channel has no **Notify** choice, because a check needs its success pings to stay green. A section with nowhere to back up to counts as failed, so its check goes red instead of hearing a success.

Sections on the same URL share one check. Once a backup of one has failed, the good backups of the others ping `/fail` as well and name it, so the check stays red until that entry has a good backup again or you acknowledge the failure under **Needs you**.

A section can ping an address of its own: **Exception** in the Healthchecks channel adds one, and the line then says **All other sections ping this URL.** **Send test** writes an entry into the log of each check, which leaves its state as it is.

## Example: Gotify on your phone {#example-gotify}

You run Gotify at `https://gotify.example.com` and want failures to ring on your phone while successes arrive quietly.

1. In Gotify, open **Apps**, create an application called `BombVault` and copy its token.
2. In BombVault, open **Settings, Notifications** and switch on **Summarise scheduled runs**, so a night of 45 backups sends one message instead of 45.
3. In **Channels**, press **Add channel** and pick **Gotify**.
4. Enter `https://gotify.example.com` as **Server URL**. BombVault adds `/message` itself.
5. Paste the token into **App token**.
6. Set **Notify** to **On success and failure**. Leave **Priority on failure** at 8 and **Priority on success** at 5. Gotify's Android app treats 8 and up as high, so only failures make a sound.
7. Switch the channel on and press **Send test**. The line under the channel says whether Gotify took it, and a refused token is named as such.

The next scheduled run sends one summary at priority 5, or at 8 when anything in it failed.

## Example: ntfy without an account {#example-ntfy}

ntfy needs no account for a public topic. Pick a topic name that is hard to guess, because anyone who knows it can read it.

1. In **Channels**, press **Add channel** and pick **ntfy**.
2. Enter the topic's address as **Topic URL**, such as `https://ntfy.sh/bombvault-tower-7f3k`, or a topic on your own ntfy server. Leave **Access token (optional)** empty for a public topic.
3. Subscribe to the same topic in the ntfy app, switch the channel on and press **Send test**.

BombVault posts the message as the body with its title in the `Title` header. A failure goes out with priority high and the tag `warning`, so it stands out in the app, and a VM left frozen goes out as urgent.

## When messages do not arrive {#troubleshooting}

See [Notifications do not arrive](troubleshooting.md#notifications-do-not-arrive) in Troubleshooting.
