# Afvigelser

Ransomware og en ødelagt app ligner hinanden i en sikkerhedskopi: Pludselig er det meste af data nyt, eller det meste er væk. BombVault sammenligner hver sikkerhedskopi med den pågældende posts egen historik og melder et fund, når noget ser forkert ud. Så længe et fund om tabte data er åbent, beholder BombVault også postens gamle sikkerhedskopier, så den sidste gode ikke kan blive for gammel. Punktet **Afvigelser** i sidepanelet vises, mens registreringen er slået til.

![Siden Afvigelser med ét kritisk fund åbent.](assets/screenshots/anomalies/page-dark.png#only-dark)
![Siden Afvigelser med ét kritisk fund åbent.](assets/screenshots/anomalies/page-light.png#only-light)

## Siden {#page}

Felterne **Kritisk**, **Advarsler** og **Bemærkninger** vælger en alvorlighed, og **Åbn** og **Lukket** over listen skifter mellem det, der er åbent, og det, der blev lukket inden for de seneste 30 dage. Et kritisk fund står på samme flade som de andre, og dets alvorlighed ses på mærket og den farvede kant. Et fund tilbyder sin løsning først, og dets detaljer viser værdien som en kurve over det sædvanlige område.

Når intet er åbent, viser **Læringsforløb** en ring med ti dele pr. post, én del for hver sikkerhedskopi, BombVault har lært af. **Overvågning** åbner listen over poster med deres egne indstillinger.

## Hvad der tjekkes {#checks}

Hver sikkerhedskopi af en container, en VM, et mappesæt, et databasedump, flashdrevet og auto-sikkerhedskopien sammenlignes med postens egen historik. Tjekkene ser på:

- de nye data, en kørsel gemte, mod de største sædvanlige mængder i de seneste sikkerhedskopier og mod den sædvanlige hastighed pr. time,
- en sikkerhedskopi, der gemte det meste af data igen, hvilket også omfatter filer, der blev omdøbt og skrevet om,
- kildens størrelse og filantal, som restic melder for hver post og hvert databasedump,
- restics egen sikkerhedskopieringstid,
- rækker af fejl og fejl, der kommer og går,
- gendannelsestjek, der holdt op med at bestå,
- den ledige plads i lokale, SFTP- og rclone-repositories, beregnet ud fra repositoryets vækst.

En post lærer af sine første 10 sikkerhedskopier. En næsten tom kilde, en omskrivning af det meste af data og fejl tjekkes fra start.

Et fund om en kildes størrelse eller filantal dateres til den første sikkerhedskopi, hvor det viste sig. **Sammenlign med kopien før** viser de mapper, hvor filer forsvandt, kom til eller blev ændret, med en note, når næsten alt ligger i et søgeindeks, en cache eller miniaturebilleder, som appen genopbygger af sig selv.

ZFS-poster overvåges datasæt for datasæt, se [ZFS-datasæt](zfs-datasets.md#anomalies). En assistent kan læse de åbne fund gennem [MCP-serveren](mcp.md#tools).

Tjekkene har grænser. S3-, B2- og REST-repositories har ingen tal for ledig plads, på Unraids brugershare er den ledige plads hele arrayets, og sikkerhedskopier fra før restic 0.17 har ingen størrelseshistorik.

## Sådan lukker du et fund {#closing}

- **Advarsler** lukker af sig selv, når årsagen er væk.
- **Kritiske** fund om tabte data og en disk, der fyldes, bliver stående, til du handler.
- **Kvitter** markerer et fund som set og lukker det. Det rejses ikke igen, før årsagen er forsvundet én gang.
- **Markér som forventet** siger, at ændringen var din. Det nye niveau bliver normalt efter 10 sikkerhedskopier, men tjekket for næsten tom kilde slås aldrig fra.
- Et ændret udvalg, for eksempel en mappe, du har sat eller fjernet flueben ved, starter postens historik forfra af sig selv.

En notifikation sendes én gang pr. forløb, og fejl og gendannelsestjek, der allerede giver besked, meldes ikke to gange.

## Gamle sikkerhedskopier beholdes imens {#retention-hold}

Mens en kilde er næsten tom, er skrumpet kraftigt eller har fået det meste af sine data gemt igen, lader opbevaring og beskæring postens gamle sikkerhedskopier være, og fundet linker til den sidste gode sikkerhedskopi, som gendannelsesvinduet så vælger på forhånd. Før du lukker sådan et fund, siger BombVault, at den næste oprydning igen sletter gamle sikkerhedskopier efter behold-reglerne. Så hvis data mangler eller blev krypteret, så gendan først og luk fundet bagefter.

## Indstillinger {#settings}

Kortet **Afvigelser** under **Indstillinger, Verificering** rummer de globale indstillinger. Hver kontrol gemmer, så snart du ændrer den, og de tre under kontakten er skjult, mens registreringen er slået fra.

| Indstilling | Standard | Hvad den gør |
|---|---|---|
| **Find afvigelser** | Til | Sammenligner hver sikkerhedskopi med postens egen historik. Er den slået fra, tjekkes intet nyt, og punktet **Afvigelser** forsvinder fra sidepanelet. Kortet linker stadig til tidligere fund. |
| **Følsomhed** | Balanceret | Streng melder mindre ændringer, Lempelig kun store. |
| **Send en notifikation for** | Kun kritiske fund | Den laveste alvorlighed, der sender en besked gennem de kanaler, du har sat op under [Notifikationer](notifications.md). Gentagne fejl i sikkerhedskopier og dumps og mislykkede planlagte gendannelsestjek sender allerede deres egen besked og sendes ikke to gange. |
| **Behold gamle sikkerhedskopier, når en kilde skrumper kraftigt eller omskrives** | Til | Mens en post har et åbent fund om en næsten tom kilde, en kraftig skrumpning eller de fleste data gemt igen, lader opbevaring og beskæring postens gamle sikkerhedskopier være. Kvitter for fundet eller markér det som forventet for at slippe dem fri. |

Hver post kan bruge sin egen følsomhed og sit eget minimum for notifikationer. Sæt dem på postens fane **Verificering** eller på siden **Afvigelser** under **Overvågning**. For en ZFS-post gælder de for hvert datasæt i dens træ.
