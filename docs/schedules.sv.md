# Scheman

**Inställningar, Schema** innehåller alla planer: den totala säkerhetskopian som täcker allt, en egen plan för ett avsnitt, de externa kopiorna, vilka poster som ingår och i vilken ordning. En enskild post kan få en egen plan på sin sida. Den här sidan förklarar hur de hänger ihop och går igenom en typisk uppsättning.

## Planer som läses som meningar {#plans}

Varje plan är en knapp som läses som en mening, till exempel *dagligen kl. 03:00* eller *varannan dag kl. 01:30*. Ett klick öppnar den med fem lägen:

| Läge | Vad du ställer in |
|---|---|
| **Av** | Ingenting körs på den här planen. |
| **Dagligen** | **Tid** |
| **Veckovis** | **Dagar** och **Tid** |
| **Var N:e dag** | **Var** så många **dagar**, och **Tid**. Det räknas från den senaste schemalagda körningen, så att säkerhetskopiera en post för hand inte skjuter planen framåt. |
| **Cron** | Ett **Uttryck** med fem fält, till exempel `0 */6 * * *` för var 6:e timme. **Exempel** erbjuder några, och fönstret visar de närmaste körningarna. |

Alla tider gäller serverns tidszon. På Unraid är det systemets egen. På andra håll ställer du in `TZ`, annars körs varje plan i UTC, se [Miljövariabler](configuration.md#environment-variables).

## Vecka {#week}

Kortet **Vecka** ritar varje planerad körning från måndag till söndag, en rad per dag: säkerhetskopior, externa kopior och kontroller. Peka på ett streck för att läsa vad som startar där. Där tre planer startar inom två timmar markeras sträckan, så att du kan sprida ut dem.

![Inställningar, Schema: bandet Vecka överst, sedan den totala säkerhetskopian och planerna per avsnitt.](assets/screenshots/schedule/week-dark.png#only-dark)
![Inställningar, Schema: bandet Vecka överst, sedan den totala säkerhetskopian och planerna per avsnitt.](assets/screenshots/schedule/week-light.png#only-light)

## Total säkerhetskopia {#full-backup}

Den **Totala säkerhetskopian** är standardplanen för allt. **När säkerhetskopiorna körs** ställer in den: ett pass som säkerhetskopierar varje avsnitt i tur och ordning, containrar, VM:ar, flash, mappar, ZFS-datauppsättningar och auto-säkerhetskopian sist. Raden under säger vilka avsnitt den omfattar. **Av** stänger av den, och då körs bara det som har en egen plan.

- **Kör ikapp missade säkerhetskopieringar efter start**: när servern var avstängd vid en planerad tid kör BombVault den säkerhetskopieringen ungefär två minuter efter att den startat.
- **Kommandon före och efter**: **Kommando före den totala säkerhetskopian** körs på servern med `sh -c` innan passet startar, till exempel ett skript som stoppar en tjänst. Om det misslyckas körs den totala säkerhetskopian ändå. **Kommando efter den totala säkerhetskopian** körs en gång efter att varje avsnitt har provats, även om något av dem misslyckades, vilket passar en ping som bekräftar att hela servern är skyddad. Kommandon för en enskild container finns på dess sida.

**Säkerhetskopiera allt nu** på sidan Säkerhetskopior startar samma pass för hand, över varje påslaget avsnitt.

## Planer per avsnitt {#plans-per-section}

Ett avsnitt kan lämna den totala säkerhetskopian och köras vid egen tid. Under **Scheman per avsnitt** lägger **Undantag** till ett avsnitt med en egen plan, och **Ta bort undantag** skickar tillbaka det. Raden på kortet säger vad de andra gör, till exempel **Alla andra avsnitt körs i den totala säkerhetskopian.**

Ett avsnitt med en egen plan säkerhetskopieras inte igen i den totala säkerhetskopian, så ingenting säkerhetskopieras två gånger. Detsamma gäller en enskild post med en egen plan.

**Externa kopior per avsnitt** fungerar på samma sätt för kopior. Som standard kopierar varje avsnitt till sina externa lagringsmål direkt efter varje säkerhetskopiering. **Undantag** ger ett avsnitt en egen kopieringsplan, till exempel så att VM:arna kopierar till Backblaze B2 först söndagsnätterna. Ett lagringsmåls eget kort **Kopiera** kan också sätta **När det kopieras**, se [Externa kopior](offsite-recovery.md#off-site-copies).

## Schemalagda poster {#scheduled-entries}

**Schemalagda poster** säger per avsnitt hur många poster som ingår, till exempel **38 av 41 schemalagda**. **Poster** öppnar listan med en omkopplare **I schemat** per post. En post som är avstängd säkerhetskopieras bara när du startar den för hand. Den räknas som oskyddad om du inte markerar den **Säkerhetskopiera inte**.

## Ordning {#order}

**Ordning** bestämmer i vilken ordning schemalagda körningar och den totala säkerhetskopian säkerhetskopierar containrar och VM:ar. Dra en post på plats eller använd pilarna, och varje flytt sparas direkt. Poster du lämnar utan ordning säkerhetskopieras med den mest försenade först. En enskild säkerhetskopiering för hand påverkas inte.

**Starta om efter säkerhetskopiering** på samma kort gäller de containrar som **Stoppa andra containrar** tog ner under en säkerhetskopiering. De startar igen i sin beroendeordning, och med **Vänta tills beroenden är felfria innan nästa startas** måste var och en rapportera frisk eller körande innan nästa startar. **Hälsotimeout per container (sekunder)** begränsar väntan, 120 som standard, i intervallet 5 till 3600.

## Ögonblicksbild i stället för stopp {#snapshot-instead-of-stop}

**Stoppa containrar på ZFS bara för en ögonblicksbild** är på som standard. En container vars data ligger på ZFS stoppas bara medan BombVault tar en ögonblicksbild av den datan, oftast några sekunder, och säkerhetskopian läses sedan från ögonblicksbilden. Det kräver SSH-anslutningen till värden som ZFS-datauppsättningar använder. En container med en del av sin data någon annanstans stoppas under hela säkerhetskopieringen som förut, och dess körning säger varför, medan en container utan data på ZFS stoppas utan en anmärkning. Det fungerar på en Unraid-användarresurs som ligger på en enda ZFS-pool. På TrueNAS behöver appen också förmågan `SYS_CHROOT`, se [TrueNAS SCALE](getting-started.md#truenas). En enskild container kan ställas in annorlunda på sin egen sida.

Säkerhetskopian hamnar under samma sökvägar och taggar som en som tagits med containern stoppad, så återställningar och kontroller behandlar båda lika. Kommandot före säkerhetskopian och databasdumpen körs fortfarande först, de andra containrarna som ska stoppas går ner och kommer tillbaka med den, och kommandot efter säkerhetskopian körs så fort containern är uppe igen. En uppdatering efter säkerhetskopian väntar fortfarande tills säkerhetskopian är klar. Dess körningar visar **Ögonblicksbild** och hur många sekunder containern var nere. Skulle BombVault stanna mitt i en containersäkerhetskopiering startar den de containrar den höll nere igen nästa gång den startar.

## Uppdateringar efter säkerhetskopian {#updates}

- **Uppdatera containrar efter en lyckad säkerhetskopiering** hämtar en nyare image först när säkerhetskopian före den är klar, så en färsk återställningspunkt finns alltid först. Det är av som standard, och en enskild container kan ställas in annorlunda på sin sida under **Uppdatera efter en lyckad säkerhetskopiering**.
- **Ta bort den gamla imagen efter en uppdatering** raderar den image som containern lämnade. En basimage som andra containrar delar raderas aldrig.
- **Uppdatera Unraids uppdateringsstatus efter att en container uppdaterats** rensar Unraids uppdateringsbanderoll för den containern.

Ett meddelande per uppdaterad container ställs in under **Inställningar, Aviseringar** med **Avisera vid containeruppdatering**.

## Vila före säkerhetskopia {#idle}

Kortet **Vila före säkerhetskopia** bestämmer när en app räknas som i vila för en container vars schemalagda säkerhetskopiering väntar på den: **CPU under (% av en kärna)**, **Trafik under (Mbit/s)** och **I minst (min)**. En mediaserver räknas som i vila när den inte strömmar, och för en container på värdnätverket räknas bara CPU. Själva väntan ställs in på varje container, se [När appen är upptagen](backups.md#app-busy).

## En egen plan för en post {#own-plan}

Varje post har **Egen plan** i kortet **Schema och mål** på sin flik Inställningar. Med en plan där körs posten vid egna tider och lämnar den totala säkerhetskopian och sitt avsnitts plan. Utan en följer den dem.

## Exempel: tre planer som passar ihop {#example}

En typisk Unraid-låda: de flesta containrar kan säkerhetskopieras på natten, VM:arna är stora och ändras långsamt, och Immich får nya foton hela dagen.

1. **Total säkerhetskopia kl. 03:00.** Öppna **Inställningar, Schema**. Sätt **När säkerhetskopiorna körs** till **Dagligen** kl. `03:00` under **Total säkerhetskopia**. Slå på **Kör ikapp missade säkerhetskopieringar efter start**, så att en natt med servern avstängd tas igen.
2. **VM:ar en gång i veckan.** Tryck på **Undantag** under **Scheman per avsnitt**, välj **VMs** och sätt dess plan till **Veckovis**, **Dagar** söndag, **Tid** `04:00`. Kortet säger nu att alla andra avsnitt körs i den totala säkerhetskopian.
3. **VM:arna kopierar externt på söndagen också.** Tryck på **Undantag** under **Externa kopior per avsnitt**, välj **VMs** och sätt **Veckovis**, söndag kl. `06:00`, när deras säkerhetskopiering har hunnit bli klar.
4. **Immich var 6:e timme.** Öppna **Säkerhetskopior**, sedan `immich`, sedan dess flik **Inställningar**. Sätt **Egen plan** till **Cron** med `0 */6 * * *`.

Det som körs nu:

| När | Vad |
|---|---|
| Varje dag kl. 03:00 | Containrar utom `immich`, flash, mappar, ZFS-datauppsättningar och auto-säkerhetskopian, ett avsnitt efter det andra |
| Söndag kl. 04:00 | VM:ar |
| Söndag kl. 06:00 | Den externa kopian av VM:arna |
| 00:00, 06:00, 12:00 och 18:00 | `immich` för sig |

Kortet **Vecka** visar 03:00-raden varje dag, strecken 04:00 och 06:00 på söndagen och fyra streck om dagen för `immich`. `immich` säkerhetskopieras inte en andra gång kl. 03:00, eftersom en post med en egen plan lämnar den totala säkerhetskopian. Kopior av de andra avsnitten körs fortfarande direkt efter varje säkerhetskopiering.

När du senare tar bort `immich`s egna plan går den med i 03:00-körningen igen tillsammans med sitt avsnitt.
