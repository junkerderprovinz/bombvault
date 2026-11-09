# Aviseringar

BombVault kan tala om för dig när en säkerhetskopiering misslyckas, när den lyckas, när en är försenad och en gång i veckan hur det har gått. Allt ställs in under **Inställningar, Aviseringar**: vad BombVault skickar utöver enskilda säkerhetskopieringar på kortet **Aviseringar** och vart meddelandena går på kortet **Kanaler**. Varje kanal bestämmer själv när den säger till, och en Healthchecks-kontroll är också en kanal.

![Inställningar, Aviseringar med Gotify-kanalen ifylld.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Inställningar, Aviseringar med Gotify-kanalen ifylld.](assets/screenshots/notifications/gotify-light.png#only-light)

## Vad som skickar ett meddelande {#what}

Omkopplarna på kortet **Aviseringar** gäller för alla kanaler:

- **Sammanfatta schemalagda körningar**: ett meddelande per schemalagd körning, till exempel *42 av 45 lyckades*, i stället för ett per container eller VM. En säkerhetskopiering du startar för hand skickar fortfarande sitt eget meddelande. En frusen gäst, en misslyckad gallring eller en överskriden tillväxtbudget går ut direkt även med sammanfattningar på.
- **Avisera vid containeruppdatering**: ett meddelande för varje container som **Uppdatera containrar efter en lyckad säkerhetskopiering** flyttade till en nyare image, så att du kan kontrollera att den fortfarande fungerar.
- **Rapportera försenade säkerhetskopior**: en gång om dagen klockan 09:00 kontrollerar BombVault om en påslagen säkerhetskopiering är försenad, vilket betyder äldre än dubbelt så gammal som dess schema, och skickar ett meddelande per fall, med namnet på posten som hamnade efter. Nästa lyckade säkerhetskopiering rensar det.
- **Veckosammanfattning**: en sammanfattning i veckan med antal körningar, ny säkerhetskopieringsdata, vilka avsnitt som är försenade, om de externa kopiorna är aktuella och de vanligaste felen. Dess plan är en knapp som varje annan plan, och **Av** stänger av den. Se [Scheman](schedules.md#plans).

Om en kanal också får höra om lyckade körningar ställs in på kanalen under **Avisera**. Vilka avvikelser som skickar ett meddelande ställs in under **Inställningar, Verifiera**, se [Avvikelser](anomalies.md). En uppskjuten säkerhetskopiering skickar ett meddelande när den börjar vänta och ett när väntan tar slut, inte ett per försök.

## Kanaler {#channels}

**Lägg till kanal** längst ned på kortet **Kanaler** erbjuder **Unraid-aviseringar**, **ntfy**, **Gotify**, **Discord**, **Slack**, **Matrix**, **E-post (SMTP)**, **Apprise**, **Webhook** och **Healthchecks**. Lägg till så många du vill, även två av samma sort, till exempel ett ntfy-ämne för fel och ett annat för allt. En ny kanal börjar avslagen, så att den sparas innan dess fält är ifyllda.

Varje kanal har:

- en egen omkopplare, som stänger av den utan att dess fält går förlorade.
- **Avisera**: **Aldrig**, **Endast vid fel** eller **Vid lyckat och fel**. En ny kanal börjar på **Endast vid fel**. En säkerhetskopiering som lyckades men hoppade över något når också en kanal som bara rapporterar fel.
- **Namn**, som är valfritt och står i stället för sorten, praktiskt när du har två av samma sort.
- **Skicka test**, som skickar ett testmeddelande genom just den här kanalen med värdena som de står i formuläret, sparade eller inte. Raden under kanalen säger om tjänsten tog emot det, och om inte, varför.
- **Ta bort kanal**, papperskorgen bredvid.

Varje kanal skickar på egen hand med sin egen tidsgräns, så en värd som aldrig svarar håller inte upp de andra. När en tjänst misslyckas på ett sätt som ett nytt försök kan rätta till, försöker kanalen upp till tre gånger. Ett larm om försening, budget eller avvikelse som ingen kanal tog emot provas igen senare, och det gör en veckosammanfattning också.

| Kanal | Fält | Anmärkningar |
|---|---|---|
| **Unraid-aviseringar** | inga | Skickar till Unraids eget aviseringssystem, som kan vidarebefordra till Pushover, e-post eller Discord. Det går över SSH-anslutningen från **Inställningar, Integrationer, Värd-SSH**, så den nyckeln måste vara auktoriserad. På TrueNAS och andra värdar förblir den tyst, och kanalen säger varför. |
| **ntfy** | **Ämnes-URL**, **Åtkomsttoken (valfritt)** | Adressen slutar med ditt ämne. En token behövs bara för ett skyddat ämne. |
| **Gotify** | **Serverns URL**, **App-token**, **Prioritet vid fel**, **Prioritet vid lyckat** | Prioriteterna går från 0 till 10 och börjar på 8 för fel och 5 för lyckade körningar. |
| **Discord** | **Webhook-URL** | Adressen till en webhook från Discord-kanalens inställningar. |
| **Slack** | **Webhook-URL** | Adressen till en inkommande webhook i din Slack-app. |
| **Matrix** | **Homeserver-URL**, **Åtkomsttoken**, **Rums-ID** | Kontot bakom token måste vara med i rummet. |
| **E-post (SMTP)** | **SMTP-värd**, **Port**, **Kryptering**, **Användarnamn**, **Lösenord**, **Avsändaradress**, **Mottagaradress** | **Kryptering** är **STARTTLS**, **TLS (implicit)** eller **None**. |
| **Apprise** | **Apprise API-URL**, **Taggar (valfritt)**, **Apprise-adresser (valfritt)** | Peka den på `/notify/<key>`-slutpunkten i din Apprise API-container för att nå dess 100+ tjänster, och taggar väljer en delmängd av den nyckelns mål. Eller lista Apprise-adresser, en per rad, till exempel `tgram://` för Telegram eller `pover://` för Pushover, och låt API-URL:en sluta på `/notify` utan nyckel. |
| **Webhook** | **Webhook-URL**, **Metod**, **Rubriker**, **Innehåll** | En begäran som du formar själv, se [Webhook](#webhook). |
| **Healthchecks** | **Ping-URL** | Se [Healthchecks](#healthchecks). |

Hemligheter som tokens, SMTP-lösenordet och värdena i webhook-rubriker lagras krypterade, och servern lämnar aldrig tillbaka dem till sidan. Under en sparad hemlighet står det i fältet **Sparad. Lämna fältet tomt för att behålla den.** När du ändrar vart en kanal skickar, går den gamla hemligheten inte till det nya stället: fältet säger **Den nya adressen sparas när du anger detta igen.**, och den nya adressen sparas så fort du skrivit den.

Inställningar från en version före kanallistan förs över vid första starten, och inget går förlorat. Varje kanal du hade fyllt i blir en egen post med det val under **Avisera** som du hade. En webhook inställd på formatet Discord, Slack, Gotify eller ntfy blir en kanal av den sorten, och en Generic JSON-webhook fortsätter att posta `title`, `message` och `ok` genom sitt **Innehåll**. Unraid-omkopplaren blir en Unraid-kanal, och Healthchecks-URL:en med sina undantag blir en Healthchecks-kanal.

### Webhook {#webhook}

En **Webhook**-kanal skickar en HTTP-begäran till valfri adress. **Metod** är GET, POST eller PUT. **Lägg till rubrik** lägger till en rad under **Rubriker**, och rubrikernas värden sparas som hemligheter, så en `Authorization`-rubrik är säker där. **Innehåll** skickas precis som du skriver det, med de här platshållarna ifyllda:

| Platshållare | Blir |
|---|---|
| `%%title%%` | meddelandets rubrik |
| `%%body%%` | själva meddelandet |
| `%%ok%%` | `true`, eller `false` vid fel |
| `%%event%%` | vad som hände, till exempel `container.failed` |
| `%%severity%%` | `info`, `success`, `warning`, `failure` eller `critical` |
| `%%importance%%` | Unraids nivå för det: `normal`, `warning` eller `alert` |
| `%%link%%` | en länk som hör till meddelandet, tom när det inte finns någon |
| `%%key%%` | avsnittet, till exempel `container` eller `VM` |
| `%%time%%` | tiden i UTC, till exempel `2026-11-09T03:00:00Z` |
| `%%source%%` | `BombVault` |

Platshållare fungerar även i adressen och i rubrikvärden, och varje värde escapas efter var det står: som JSON när rubriken `Content-Type` säger JSON, och som frågevärde i adressen. En ny webhook börjar med den rubriken och postar `{"title":"%%title%%","message":"%%body%%","ok":%%ok%%}`. Ett namn som BombVault inte känner till står kvar som det är skrivet, så ett stavfel syns i meddelandet. Lämna **Innehåll** tomt för att inte skicka något innehåll.

## Healthchecks {#healthchecks}

En tjänst som [Healthchecks.io](https://healthchecks.io) slår larm när BombVault slutar checka in. Lägg till en **Healthchecks**-kanal och klistra in kontrollens **Ping-URL**, så pingar BombVault den under hela livscykeln för varje säkerhetskopiering: vid start, sedan vid lyckat resultat eller med `/fail`. Den här kanalen har inget val under **Avisera**, eftersom en kontroll behöver sina pingar för lyckade körningar för att förbli grön. Ett avsnitt som inte har någonstans att säkerhetskopiera till räknas som misslyckat, så dess kontroll blir röd i stället för att få höra om ett lyckat resultat.

Avsnitt med samma URL delar en kontroll. När en säkerhetskopiering av ett av dem har misslyckats pingar även de lyckade säkerhetskopiorna av de andra `/fail` och nämner det, så kontrollen förblir röd tills den posten har en lyckad säkerhetskopia igen eller du kvitterar felet under **Behöver uppmärksamhet**.

Ett avsnitt kan pinga en egen adress: **Undantag** i Healthchecks-kanalen lägger till en, och raden säger då **Alla andra avsnitt pingar den här URL:en.** **Skicka test** skriver en post i loggen för varje kontroll, vilket lämnar dess status som den är.

## Exempel: Gotify i telefonen {#example-gotify}

Du kör Gotify på `https://gotify.example.com` och vill att fel ska ringa i telefonen medan lyckade körningar kommer tyst.

1. Öppna **Appar** i Gotify, skapa en applikation som heter `BombVault` och kopiera dess token.
2. Öppna **Inställningar, Aviseringar** i BombVault och slå på **Sammanfatta schemalagda körningar**, så att en natt med 45 säkerhetskopior skickar ett meddelande i stället för 45.
3. Tryck på **Lägg till kanal** under **Kanaler** och välj **Gotify**.
4. Ange `https://gotify.example.com` som **Serverns URL**. BombVault lägger själv till `/message`.
5. Klistra in token i **App-token**.
6. Sätt **Avisera** till **Vid lyckat och fel**. Låt **Prioritet vid fel** stå på 8 och **Prioritet vid lyckat** på 5. Gotifys Android-app behandlar 8 och uppåt som hög, så bara fel ger ljud.
7. Slå på kanalen och tryck på **Skicka test**. Raden under kanalen säger om Gotify tog emot det, och en nekad token nämns som sådan.

Nästa schemalagda körning skickar en sammanfattning med prioritet 5, eller 8 om något i den misslyckades.

## Exempel: ntfy utan konto {#example-ntfy}

ntfy behöver inget konto för ett publikt ämne. Välj ett ämnesnamn som är svårt att gissa, eftersom alla som känner till det kan läsa det.

1. Tryck på **Lägg till kanal** under **Kanaler** och välj **ntfy**.
2. Ange ämnets adress som **Ämnes-URL**, till exempel `https://ntfy.sh/bombvault-tower-7f3k`, eller ett ämne på din egen ntfy-server. Lämna **Åtkomsttoken (valfritt)** tomt för ett publikt ämne.
3. Prenumerera på samma ämne i ntfy-appen, slå på kanalen och tryck på **Skicka test**.

BombVault postar meddelandet som kroppen med titeln i rubriken `Title`. Ett fel går ut med hög prioritet och taggen `warning`, så det syns tydligt i appen, och en VM som blir kvar fryst går ut som brådskande.

## När meddelanden inte kommer fram {#troubleshooting}

Se [Aviseringar kommer inte fram](troubleshooting.md#notifications-do-not-arrive) i Felsökning.
