# Översikt

Översikten är första sidan i sidofältet och den du bör kolla varje morgon. Den svarar på två frågor med en blick: är allt säkerhetskopierat, och är det något som behöver dig? Den börjar med ett statushuvud och fyra kort, och fler kort väntar under **Anpassa**.

![Översikten efter några dagar: statushuvudet, Behöver uppmärksamhet med en varning, skyddet för varje avsnitt, lagringsmålen och historiken.](assets/screenshots/overview/home-dark.png#only-dark)
![Översikten efter några dagar: statushuvudet, Behöver uppmärksamhet med en varning, skyddet för varje avsnitt, lagringsmålen och historiken.](assets/screenshots/overview/home-light.png#only-light)

## Statushuvudet {#status-head}

Huvudet ligger överst på sidan och stannar alltid kvar. Ringen visar hur mycket som är skyddat, till exempel **41 av 43 skyddade**, och raden bredvid säger hur läget är:

| Huvudet säger | Vad det betyder |
|---|---|
| **Allt är säkerhetskopierat** | Varje post i varje påslaget avsnitt har en aktuell säkerhetskopia. |
| **Säkerhetskopierat, med anmärkningar** | Allt är säkerhetskopierat, och några körningar lämnade en anmärkning, till exempel filer de inte kunde läsa. |
| **Allt är inte skyddat ännu** | Vissa poster saknar en aktuell säkerhetskopia, till exempel för att de väntar på sin första. |
| **3 saker behöver uppmärksamhet** | Något har misslyckats eller väntar på ett beslut. Kortet **Behöver uppmärksamhet** listar det. |
| **Säkerhetskopierar nu** | En säkerhetskopiering pågår. Ringen blir en förloppsring med stationerna **Stopp**, **Läs** och **Start**, och en full säkerhetskopiering räknar sina poster medan den går, till exempel **Säkerhetskopierar nu: 12 av 43**. **Visa körning** öppnar körningen. |
| **Extern kopiering pågår** | En kopiering till ett mål pågår. Den visar ingen procentsats, eftersom `restic copy` inte rapporterar någon. |

Under raden står när den senaste körningen var och när nästa planerade startar.

## Behöver uppmärksamhet {#needs-you}

Det här kortet listar det du bör titta på, det nyaste först, och säger **Inget att göra.** när listan är tom. Varje rad har sin egen väg ut:

- **En misslyckad säkerhetskopiering** nämner posten och orsaken. **Visa fel** visar restics egen utdata, och **Försök igen** startar säkerhetskopieringen på nytt.
- **Extern kopiering misslyckades** nämner avsnittet och sedan när. **Replikera nu** försöker igen.
- **Avvikelse vid plex** leder till fyndet med **Kontrollera**. Så länge ett kritiskt fynd är öppet behåller gallringen den postens gamla säkerhetskopior.
- **Nya** poster som BombVault hittat och som ingen säkerhetskopierar än, till exempel en container du nyss installerat, erbjuder **Börja säkerhetskopiera** eller **Säkerhetskopiera inte**.
- **En säkerhetskopiering som gav upp** eftersom appen var upptagen under hela väntetiden, se [När appen är upptagen](backups.md#app-busy).
- **En misslyckad ZFS-replik**, i klartext, till exempel att servern saknar kommandot zfs, med serverns utdata under **Visa**.
- **Säkerhetskopieringen står still: nyckeln går inte längre att öppna** visas när den här servern tog över en annan servers nyckel och `APP_KEY` ändrades efteråt. Se [Felsökning](troubleshooting.md#backups-stopped-and-say-the-key-can-no-longer-be-opened).
- **Spara ditt återställningskit** ligger kvar tills du har laddat ner kitet och tryckt på **Jag har förvarat det säkert**.

## Skyddsstatus {#protection-status}

Varje påslaget avsnitt har en rad med fyra kontroller:

| Kontroll | Den blir grön när | Andra ord du kan se |
|---|---|---|
| **Lokal** | den senaste säkerhetskopian är aktuell | ingen säkerhetskopia än, inget schema, senaste körningen misslyckades, säkerhetskopian är försenad, inte varje post är skyddad, säkerhetskopia snart |
| **Extern** | den senaste kopian utanför lokalen är aktuell | ingen extern kopia, kopiering pågår, kopian är försenad, väntar på ditt beslut, ingen kopia än, repliken misslyckades |
| **Skydd mot radering** | ett manipuleringstest har visat att andra sidan vägrar radering | inget, inget och risken accepterad, inte bevisat än, testet misslyckades, inte bevisat eftersom kontrollen är av |
| **Teståterställning** | en återställning från den externa kopian lyckades | aldrig körd, pågår, misslyckades, försenad, bara manuellt, inte möjligt för det här avsnittet |

Ett avsnitt är grönt bara på kontrollerade fakta. En kopia på den här servern räknas inte som extern, och ett mål som du markerat **Acceptera utan skydd mot radering** får en grå ring i stället för en gul punkt, så avsnittets färg följer dess andra kontroller.

Ett schemalagt avsnitt visar **Ingen säkerhetskopia ännu** och förblir neutralt tills det är dags för dess första körning, räknat från när dess plan senast ställdes in. Därefter står det **Försenad** och det larmar en gång. **Försenad** utgår från den post som har hamnat längst efter sin egen plan, så att en granne som körs inte döljer en som har stannat. En post vars container är borta eller som inget schema når räknas inte. En post med en egen veckoplan hålls inte till avsnittets dagliga plan. Ett avsnitt utan något att säkerhetskopiera, eftersom inget schema når någon av dess poster, visar **Ej schemalagd** och är aldrig försenat.

Ett klick på ett avsnitt öppnar dess fönster med varje kontroll utskriven, de poster den räknar och **Starta teståterställning** eller **Kör teståterställningen igen**. Det visar också **Rensningsregel angiven** eller **Ingen rensningsregel**, och behållregeln för ett avsnitts egna mapp räknas lika väl som dess externa regler.

![Skyddsfönstret för avsnittet Containers med sina fyra kontroller utskrivna.](assets/screenshots/overview/protection-window-dark.png#only-dark)
![Skyddsfönstret för avsnittet Containers med sina fyra kontroller utskrivna.](assets/screenshots/overview/protection-window-light.png#only-light)

## Lagringsmål {#storage-locations}

Varje lagringsmål står här som ett kärl med sin fyllnadsgrad, hur snabbt det växer, till exempel **Växer 4,2 GB/vecka**, och när det skulle vara fullt i den takten, till exempel **full om ~31 veckor**. S3, B2 och rest-servrar rapporterar inte hur mycket utrymme som finns kvar, så de visar **Ledigt utrymme okänt** och bara sin tillväxt. Ett klick leder till målets sida. Se [Lagringsmål](offsite-recovery.md#destinations).

## Historik {#history}

**Historik** har två flikar. **Körningar** listar varje körning med resultat, varaktighet och orsak, och ett klick öppnar den. **Logg** visar varje rad med sin tid, som BombVault skriver den, och kan begränsas till en dag. En rad blir orange när en säkerhetskopiering gick igenom men lämnade något utanför, och raden nämner vad som lämnades utanför.

## Fler kort under Anpassa {#customize}

**Anpassa** högst upp på sidan låter dig ordna korten. Dra ett kort i dess list för att flytta det, ändra storleken vid punkten i dess nedre hörn, eller dölj det. Dolda kort väntar under **Dolda kort**, och **Visa** tar tillbaka ett. Medan du ordnar dem gungar korten lite, och **Klar** avslutar det. Layouten sparas i den här webbläsaren.

De här korten börjar dolda:

- **Avvikelser**: de öppna fynden, se [Avvikelser](anomalies.md).
- **Inte säkerhetskopierat**: allt på den här servern som ingen schemalagd säkerhetskopiering täcker, vart och ett med sin orsak, till exempel **aldrig tillagd**, **ingår inte i schemat** eller **alla mål i dess avsnitt är avstängda**. Det läser containrar direkt från Docker, så det hittar också en som BombVault aldrig sett.
- **Kalender**: de senaste 90 dagarna, en ruta per dag. Ett klick på en dag visar dess rader i loggen under **Historik**.
- **Lagring**: hur mycket varje avsnitt tar upp och hur väl dess data dedupliceras.
- **Översikt**: den äldsta ögonblicksbilden i alla avsnitt, när nästa säkerhetskopiering körs och hur den senaste gick.
- **Statistik**: hur många containrar och VM:ar det finns, hur många fel, och hur många planer som är aktiva eller pausade.
- **Värdintegration**: vad BombVault når och behöver på värden, till exempel Docker-socketen, libvirt, restic och rclone. Börja här när något verkar vara osammankopplat.

![Läget Anpassa: korten kan dras, ändra storlek och döljas, och de dolda väntar nedanför.](assets/screenshots/overview/customize-dark.png#only-dark)
![Läget Anpassa: korten kan dras, ändra storlek och döljas, och de dolda väntar nedanför.](assets/screenshots/overview/customize-light.png#only-light)

## Så räknar BombVault {#counting}

Varje kort och huvudet räknar på samma sätt: posterna i de påslagna avsnitten som fortfarande är installerade och som du inte markerat **Säkerhetskopiera inte**. En post räknas som skyddad bara medan ett schema når den och den har en säkerhetskopia. En gammal säkerhetskopia ensam räknas inte, så en container som du tagit ur schemat visas som oskyddad tills du lägger tillbaka den eller markerar den **Säkerhetskopiera inte**.

## Första stegen {#first-steps}

På en ny installation börjar översikten med kortet **Första stegen**: **Välj avsnitt**, **Lägg till mål** och **Första säkerhetskopian**. Det ligger kvar tills den första säkerhetskopian är klar. [Kom igång](getting-started.md#first-run) går igenom det.
