# Avvik

Løsepengevirus og en ødelagt app ser like ut i en sikkerhetskopi: plutselig er det meste av dataene nytt, eller det meste er borte. BombVault sammenligner hver sikkerhetskopi med historikken til det aktuelle elementet og melder et funn når noe ser galt ut. Så lenge et funn om tapte data er åpent, beholder BombVault også de gamle sikkerhetskopiene til elementet, slik at den siste gode ikke kan eldes bort. Oppføringen **Avvik** i sidefeltet vises så lenge oppdagelsen er på.

![Siden Avvik med ett kritisk funn åpent.](assets/screenshots/anomalies/page-dark.png#only-dark)
![Siden Avvik med ett kritisk funn åpent.](assets/screenshots/anomalies/page-light.png#only-light)

## Siden {#page}

Flisene **Kritisk**, **Advarsler** og **Merknader** velger alvorlighetsgrad, og **Åpent** og **Lukket** over listen bytter mellom det som er åpent og det som er lukket de siste 30 dagene. Et kritisk funn ligger på samme flate som de andre, og alvorligheten vises i merket og i den fargede kanten. Et funn tilbyr løsningen først, og detaljene viser verdien som en kurve over det vanlige området.

Når ingenting er åpent, viser **Læringsfremdrift** en ring med ti deler per element, én del for hver sikkerhetskopi BombVault har lært av. **Overvåking** åpner listen over elementer med egne innstillinger.

## Hva som kontrolleres {#checks}

Hver sikkerhetskopi av en container, VM, mappesamling, databasedump, flash-stasjonen og selvsikkerhetskopien sammenlignes med elementets egen historikk. Kontrollene ser på:

- de nye dataene en kjøring lagret, mot de største vanlige mengdene i nyere sikkerhetskopier og mot den vanlige raten per time,
- en sikkerhetskopi som lagret det meste av dataene på nytt, også filer som ble omdøpt og skrevet om,
- kildestørrelsen og filantallet restic melder for hvert element og hver databasedump,
- restics egen sikkerhetskopitid,
- rekker av feil og feil som kommer og går,
- gjenopprettingskontroller som sluttet å bestå,
- ledig plass i lokale depoter, SFTP-depoter og rclone-depoter, beregnet ut fra hvor fort depotet vokser.

Et element lærer av sine 10 første sikkerhetskopier. En nesten tom kilde, en omskriving av det meste av dataene og feil kontrolleres fra første stund.

Et funn om størrelsen eller filantallet til en kilde dateres til den første sikkerhetskopien det viste seg i. **Sammenlign med kopien før** lister mappene der filer forsvant, kom til eller endret seg, med en merknad når nesten alt ligger i en søkeindeks, en hurtigbuffer eller miniatyrbilder som appen bygger opp igjen av seg selv.

ZFS-elementer overvåkes datasett for datasett, se [ZFS-datasett](zfs-datasets.md#anomalies). En assistent kan lese de åpne funnene via [MCP-serveren](mcp.md#tools).

Kontrollene har grenser. S3-, B2- og REST-depoter har ingen tall for ledig plass, på Unraids brukerdeling er den ledige plassen den til hele arrayet, og sikkerhetskopier laget før restic 0.17 har ingen størrelseshistorikk.

## Lukke et funn {#closing}

- **Advarsler** lukkes av seg selv når årsaken er borte.
- **Kritiske** funn om tapte data og en fylt disk blir stående til du handler.
- **Kvitter** markerer et funn som sett og lukker det. Det meldes ikke på nytt før årsaken har vært borte én gang.
- **Merk som forventet** sier at endringen var din. Det nye nivået blir normalt etter 10 sikkerhetskopier, men kontrollen for nesten tom kilde slås aldri av.
- Et endret utvalg, for eksempel en mappe du har krysset av eller fjernet krysset for, starter elementets historikk på nytt av seg selv.

Et varsel sendes én gang per hendelsesforløp, og feil og gjenopprettingskontroller som allerede sender varsel, meldes ikke to ganger.

## Gamle sikkerhetskopier beholdes i mellomtiden {#retention-hold}

Mens en kilde er nesten tom, har krympet kraftig eller har fått det meste av dataene lagret på nytt, lar oppbevaring og beskjæring elementets gamle sikkerhetskopier være i fred, og funnet lenker til den siste gode sikkerhetskopien, som gjenopprettingsvinduet så forhåndsvelger. Før du lukker et slikt funn, sier BombVault fra at neste opprydding igjen sletter gamle sikkerhetskopier etter beholdningsreglene. Mangler det data, eller ble de kryptert, gjenoppretter du derfor først og lukker funnet etterpå.

## Innstillinger {#settings}

Kortet **Avvik** under **Innstillinger, Verifisering** har de globale innstillingene. Hver kontroll lagres så snart du endrer den, og de tre under bryteren skjules mens oppdagelsen er av.

| Innstilling | Standard | Hva den gjør |
|---|---|---|
| **Oppdag avvik** | På | Sammenligner hver sikkerhetskopi med elementets egen historikk. Slått av sjekkes ingenting nytt, og oppføringen **Avvik** forsvinner fra sidefeltet. Kortet lenker fortsatt til tidligere funn. |
| **Følsomhet** | Balansert | Streng melder mindre endringer, Romslig bare store. |
| **Send varsel for** | Bare kritiske funn | Laveste alvorlighetsgrad som sender en melding gjennom kanalene som er satt opp under [Varsler](notifications.md). Gjentatte feil ved sikkerhetskopiering og dumping og mislykkede planlagte gjenopprettingskontroller sender allerede sin egen melding og sendes ikke to ganger. |
| **Behold gamle sikkerhetskopier når en kilde krymper kraftig eller skrives om** | På | Mens et element har et åpent funn om en nesten tom kilde, en kraftig krymping eller at det meste av dataene er lagret på nytt, lar oppbevaring og beskjæring elementets gamle sikkerhetskopier være i fred. Kvitter funnet eller merk det som forventet for å slippe dem fri. |

Hvert element kan ha sin egen følsomhet og sitt eget minstenivå for varsler. Sett dem på elementets fane **Verifisering**, eller på siden **Avvik** under **Overvåking**. For et ZFS-element gjelder de for hvert datasett i treet.
