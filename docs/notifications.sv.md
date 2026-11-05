# Aviseringar

BombVault kan tala om för dig när en säkerhetskopiering misslyckas, när den lyckas, när en är försenad och en gång i veckan hur det har gått. Allt ställs in under **Inställningar, Aviseringar**: vad som utlöser ett meddelande på kortet **Aviseringar**, vart meddelandena går på kortet **Kanaler** och incheckningspingarna på kortet **Healthchecks**.

![Inställningar, Aviseringar med Gotify-kanalen ifylld.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Inställningar, Aviseringar med Gotify-kanalen ifylld.](assets/screenshots/notifications/gotify-light.png#only-light)

## Vad som skickar ett meddelande {#what}

Kortet **Aviseringar** sätter reglerna för alla kanaler på en gång:

- **Avisera**: **Aldrig**, **Endast vid fel** eller **Vid lyckat och fel**, för varje kanal nedan. En ny installation börjar på **Endast vid fel**, så att en kanal du fyller i säger till när en säkerhetskopiering går sönder.
- **Sammanfatta schemalagda körningar**: ett meddelande per schemalagd körning, till exempel *42 av 45 lyckades*, i stället för ett per container eller VM. En säkerhetskopiering du startar för hand skickar fortfarande sitt eget meddelande. En frusen gäst, en misslyckad gallring eller en överskriden tillväxtbudget går ut direkt även med sammanfattningar på.
- **Avisera vid containeruppdatering**: ett meddelande för varje container som **Uppdatera containrar efter en lyckad säkerhetskopiering** flyttade till en nyare image, så att du kan kontrollera att den fortfarande fungerar.
- **Rapportera försenade säkerhetskopior**: en gång om dagen klockan 09:00 kontrollerar BombVault om en påslagen säkerhetskopiering är försenad, vilket betyder äldre än dubbelt så gammal som dess schema, och skickar ett meddelande per fall, med namnet på posten som hamnade efter. Nästa lyckade säkerhetskopiering rensar det.
- **Unraid-aviseringar**: skickar till Unraids eget aviseringssystem, som kan vidarebefordra till Pushover, e-post eller Discord. Det går över SSH-anslutningen från **Inställningar, Integrationer, Värd-SSH**, så den nyckeln måste vara auktoriserad. På TrueNAS och andra värdar förblir det av.
- **Veckosammanfattning**: en sammanfattning i veckan med antal körningar, ny säkerhetskopieringsdata, vilka avsnitt som är försenade, om de externa kopiorna är aktuella och de vanligaste felen. Dess plan är en knapp som varje annan plan, och **Av** stänger av den. Se [Scheman](schedules.md#plans).

Vilka avvikelser som skickar ett meddelande ställs in under **Inställningar, Verifiera**, se [Avvikelser](anomalies.md). En uppskjuten säkerhetskopiering skickar ett meddelande när den börjar vänta och ett när väntan tar slut, inte ett per försök.

**Skicka test** längst ned på kortet skickar ett testmeddelande genom varje påslagen kanal, med värdena som de står i formuläret, och nämner varje kanal som misslyckades. En Healthchecks-kontroll får en post i sin logg, vilket lämnar dess status som den är.

## Kanaler {#channels}

Varje kanal har en egen omkopplare på kortet **Kanaler**, och dess fält visas när den är på. Varje kanal skickar på egen hand med sin egen tidsgräns, så en värd som aldrig svarar håller inte upp de andra. Ett larm om försening, budget eller avvikelse som ingen kanal tog emot provas igen, och det gör en veckosammanfattning också. Meddelandet för en enskild säkerhetskopiering eller en sammanfattning skickas en gång.

| Kanal | Fält | Anmärkningar |
|---|---|---|
| **Webhook** | **Webhook-URL**, **Webhook-format** | Format: **Generic JSON**, **Discord**, **Slack**, **Gotify** och **ntfy**. Generic JSON postar `title`, `message` och `ok`. |
| **Apprise** | **Apprise API-URL**, **Taggar (valfritt)** | Peka den på `/notify/<key>`-slutpunkten i din Apprise API-container för att nå dess 100+ tjänster. Taggar väljer en delmängd av den nyckelns mål. |
| **Matrix** | **Homeserver-URL**, **Åtkomsttoken**, **Rums-ID** | Token stannar på servern och visas aldrig igen. |
| **Gotify** | **Serverns URL**, **App-token**, **Prioritet vid fel**, **Prioritet vid lyckat** | Prioriteterna går från 0 till 10 och börjar på 8 för fel och 5 för lyckade körningar. |
| **E-post (SMTP)** | **SMTP-värd**, **Port**, **Kryptering**, **Användarnamn**, **Lösenord**, **Avsändaradress**, **Mottagaradress** | **Kryptering** är **STARTTLS**, **TLS (implicit)** eller **Ingen**. |

Hemligheter som Matrix-token, Gotifys app-token och SMTP-lösenordet lagras krypterade. Under en sparad hemlighet står det i fältet **Sparad. Lämna fältet tomt för att behålla den.** När du ändrar serverns adress skickar BombVault inte den gamla hemligheten till det nya stället: fältet frågar efter den igen, och den nya adressen sparas när du skrivit den.

## Healthchecks {#healthchecks}

En tjänst som [Healthchecks.io](https://healthchecks.io) slår larm när BombVault slutar checka in. Klistra in kontrollens **Ping-URL**, så pingar BombVault den under hela livscykeln för varje säkerhetskopiering: vid start, sedan vid lyckat resultat eller med `/fail`. Det gör den så fort en URL är satt, oavsett vad **Avisera** säger, så kontrollen förblir grön vid lyckat resultat även med meddelanden bara vid fel. Ett avsnitt som inte har någonstans att säkerhetskopiera till räknas som misslyckat, så dess kontroll blir röd i stället för att få höra om ett lyckat resultat.

Avsnitt med samma URL delar en kontroll. När en säkerhetskopiering av ett av dem har misslyckats pingar även de lyckade säkerhetskopiorna av de andra `/fail` och nämner det, så kontrollen förblir röd tills den posten har en lyckad säkerhetskopia igen eller du kvitterar felet under **Behöver uppmärksamhet**.

Ett avsnitt kan pinga en egen adress: **Undantag** på kortet lägger till en, och raden säger **Alla andra avsnitt pingar den här URL:en.**

## Exempel: Gotify i telefonen {#example-gotify}

Du kör Gotify på `https://gotify.example.com` och vill att fel ska ringa i telefonen medan lyckade körningar kommer tyst.

1. Öppna **Appar** i Gotify, skapa en applikation som heter `BombVault` och kopiera dess token.
2. Öppna **Inställningar, Aviseringar** i BombVault. Sätt **Avisera** till **Vid lyckat och fel** på kortet **Aviseringar** och slå på **Sammanfatta schemalagda körningar**, så att en natt med 45 säkerhetskopior skickar ett meddelande i stället för 45.
3. Slå på **Gotify** under **Kanaler**.
4. Ange `https://gotify.example.com` som **Serverns URL**. BombVault lägger själv till `/message`.
5. Klistra in token i **App-token**.
6. Låt **Prioritet vid fel** stå på 8 och **Prioritet vid lyckat** på 5. Gotifys Android-app behandlar 8 och uppåt som hög, så bara fel ger ljud.
7. Tryck på **Skicka test**. Raden under kortet säger om Gotify tog emot det, och en nekad token nämns som sådan.

Nästa schemalagda körning skickar en sammanfattning med prioritet 5, eller 8 om något i den misslyckades.

## Exempel: ntfy som webhook {#example-ntfy}

ntfy behöver inget konto för ett publikt ämne. Välj ett ämnesnamn som är svårt att gissa, eftersom alla som känner till det kan läsa det.

1. Slå på **Webhook** under **Kanaler**.
2. Ange ämnets adress som **Webhook-URL**, till exempel `https://ntfy.sh/bombvault-tower-7f3k`, eller adressen till din egen ntfy-server.
3. Sätt **Webhook-format** till **ntfy**.
4. Prenumerera på samma ämne i ntfy-appen och tryck på **Skicka test**.

BombVault postar meddelandet som kroppen med dess rubrik i `X-Title`. Ett fel går ut med hög prioritet och taggen `warning`, så det syns tydligt i appen.

## När meddelanden inte kommer fram {#troubleshooting}

Se [Aviseringar kommer inte fram](troubleshooting.md#notifications-do-not-arrive) i Felsökning.
