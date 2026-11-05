# Oversikt

Oversikten er den første siden i sidefeltet og den du sjekker hver morgen. Den svarer på to spørsmål med ett blikk: er alt sikkerhetskopiert, og er det noe som trenger deg? Den starter med et statushode og fire kort, og flere kort venter under **Tilpass**.

![Oversikten etter noen dager: statushodet, Krever oppmerksomhet med én advarsel, beskyttelsen av hvert område, målene og historikken.](assets/screenshots/overview/home-dark.png#only-dark)
![Oversikten etter noen dager: statushodet, Krever oppmerksomhet med én advarsel, beskyttelsen av hvert område, målene og historikken.](assets/screenshots/overview/home-light.png#only-light)

## Statushodet {#status-head}

Hodet ligger øverst på siden og blir der alltid. Ringen viser hvor mye som er beskyttet, for eksempel **41 av 43 beskyttet**, og linjen ved siden av sier hvordan det står til:

| Hodet sier | Hva det betyr |
|---|---|
| **Alt er sikkerhetskopiert** | Hvert element i hvert område som er slått på, har en oppdatert sikkerhetskopi. |
| **Sikkerhetskopiert, med merknader** | Alt er sikkerhetskopiert, og noen kjøringer etterlot en merknad, for eksempel filer de ikke kunne lese. |
| **Ikke alt er beskyttet ennå** | Noen elementer har ingen oppdatert sikkerhetskopi, for eksempel fordi de venter på sin første. |
| **3 ting krever oppmerksomhet** | Noe feilet eller venter på en avgjørelse. Kortet **Krever oppmerksomhet** lister det. |
| **Sikkerhetskopierer nå** | En sikkerhetskopi kjører. Ringen blir til en fremdriftsring med stasjonene **Stopp**, **Les** og **Start**, og en full sikkerhetskopi teller elementene underveis, for eksempel **Sikkerhetskopierer nå: 12 av 43**. **Vis kjøring** åpner kjøringen. |
| **Off-site-kopi pågår** | En kopi til et mål kjører. Den viser ingen prosent, fordi `restic copy` ikke melder noen. |

Under linjen står det når siste kjøring var og når den neste planlagte starter.

## Krever oppmerksomhet {#needs-you}

Kortet lister det du bør se på, nyeste først, og sier **Ingenting å gjøre.** når listen er tom. Hver rad har sin egen vei ut:

- **En mislykket sikkerhetskopi** nevner elementet og årsaken. **Vis feil** viser restics egen utskrift, og **Prøv igjen** starter sikkerhetskopien på nytt.
- **Off-site-kopi mislyktes** nevner området og siden når. **Replikér nå** prøver igjen.
- **Avvik ved plex** fører til funnet med **Sjekk**. Mens et kritisk funn er åpent, beholder oppbevaringen elementets gamle sikkerhetskopier.
- **Nye** elementer som BombVault har funnet og som ingenting sikkerhetskopierer ennå, for eksempel en container du nettopp installerte, tilbyr **Begynn å sikkerhetskopiere** eller **Ikke sikkerhetskopier**.
- **En sikkerhetskopi som ga opp** fordi appen var opptatt hele ventetiden, se [Når appen er opptatt](backups.md#app-busy).
- **En mislykket ZFS-replika**, i vanlige ord, for eksempel at serveren ikke har noen zfs-kommando, med serverens utskrift under **Vis**.
- **Sikkerhetskopieringen står stille: nøkkelen kan ikke lenger åpnes** vises når denne serveren overtok nøkkelen til en annen server og `APP_KEY` ble endret etterpå. Se [Feilsøking](troubleshooting.md#backups-stopped-and-say-the-key-can-no-longer-be-opened).
- **Lagre gjenopprettingssettet ditt** blir stående til du har lastet ned settet og trykket på **Jeg har lagret det trygt**.

## Beskyttelsesstatus {#protection-status}

Hvert område som er slått på har en rad med fire kontroller:

| Kontroll | Den blir grønn når | Andre ord du kan se |
|---|---|---|
| **Lokal** | siste sikkerhetskopi er oppdatert | ingen sikkerhetskopi ennå, ingen tidsplan, siste kjøring feilet, sikkerhetskopi forsinket, ikke alle elementer er beskyttet, sikkerhetskopi snart påkrevd |
| **Off-site** | siste kopi utenfor huset er oppdatert | ingen off-site-kopi, kopi pågår, kopi forsinket, venter på avgjørelsen din, ingen kopi ennå, replikering mislyktes |
| **Beskyttelse mot sletting** | en tamper-test beviste at den andre siden nekter sletting | ingen, ingen og risiko akseptert, ikke bevist ennå, testen feilet, ikke bevist fordi kontrollen er slått av |
| **Prøvegjenoppretting** | en gjenoppretting fra off-site-kopien bestod | aldri kjørt, pågår, feilet, forsinket, kun manuelt, ikke mulig for dette området |

Et område er grønt bare på bekreftede fakta. En kopi på denne serveren regnes ikke som off-site, og et mål du merket **Godta uten beskyttelse mot sletting** viser en grå ring i stedet for en gul prikk, så områdets farge følger de andre kontrollene.

Et planlagt område viser **Ingen sikkerhetskopi ennå** og forblir nøytralt til tidspunktet for den første kjøringen er nådd, regnet fra da planen sist ble satt. Etter det står det **Forfalt**, og det varsler én gang. **Forfalt** går etter elementet som har kommet lengst etter sin egen plan, så en nabo som kjører, ikke skjuler en som har stoppet. Et element der containeren er borte, eller som ingen tidsplan når, teller ikke. Et element med en egen ukentlig plan holdes ikke til den daglige planen til området. Et område uten noe å sikkerhetskopiere, fordi ingen tidsplan når noen av elementene, viser **Ikke planlagt** og er aldri forfalt.

Et klikk på et område åpner vinduet med alle kontrollene forklart, elementene det teller og **Start testgjenoppretting** eller **Kjør testgjenoppretting på nytt**. Det viser også **Beskjæringsregel satt** eller **Ingen beskjæringsregel**, og oppbevaringsregelen for et områdes egen mappe teller like mye som off-site-reglene.

![Beskyttelsesvinduet for området Kontainere med de fire kontrollene forklart.](assets/screenshots/overview/protection-window-dark.png#only-dark)
![Beskyttelsesvinduet for området Kontainere med de fire kontrollene forklart.](assets/screenshots/overview/protection-window-light.png#only-light)

## Mål {#storage-locations}

Hvert mål står her som et kar med fyllingsgrad, hvor fort det vokser, for eksempel **Vokser 4,2 GB/uke**, og når det ville vært fullt med den farten, for eksempel **fullt om ~31 uker**. S3, B2 og rest-servere oppgir ikke hvor mye plass som er igjen, så de viser **Ledig plass ukjent** og bare veksten. Et klikk fører til målets side. Se [Mål](offsite-recovery.md#destinations).

## Historikk {#history}

**Historikk** har to faner. **Kjøringer** lister hver kjøring med resultat, varighet og årsak, og et klikk åpner den. **Logg** viser hver linje med klokkeslett, slik BombVault skriver den, og kan avgrenses til én dag. En linje blir oransje når en sikkerhetskopi gikk gjennom men utelot noe, og linjen sier hva den utelot.

## Flere kort under Tilpass {#customize}

**Tilpass** øverst på siden lar deg ordne kortene. Dra et kort i linjen for å flytte det, endre størrelsen med prikken i det nederste hjørnet, eller skjul det. Skjulte kort venter under **Skjulte kort**, og **Vis** henter ett tilbake. Mens du ordner dem, vugger kortene litt, og **Ferdig** avslutter det. Oppsettet lagres i denne nettleseren.

Disse kortene starter skjult:

- **Avvik**: de åpne funnene, se [Avvik](anomalies.md).
- **Ikke sikkerhetskopiert**: alt på denne serveren som ingen planlagt sikkerhetskopi dekker, hver med sin årsak, for eksempel **aldri lagt til**, **ikke med i tidsplanen** eller **alle mål i området er slått av**. Kortet leser containere fra Docker selv, så det finner også en som BombVault aldri har sett.
- **Kalender**: de siste 90 dagene, én rute per dag. Et klikk på en dag viser linjene i loggen under **Historikk**.
- **Lagring**: hvor mye plass hvert område tar og hvor godt dataene dedupliseres.
- **Oversikt**: den eldste sikkerhetskopien på tvers av alle områder, når neste sikkerhetskopi kjører og hvordan den siste gikk.
- **Statistikk**: hvor mange containere og VM-er det finnes, hvor mange feil, og hvor mange planer som er aktive eller pauset.
- **Host-integrasjon**: hva BombVault når og trenger på verten, for eksempel Docker-socketen, libvirt, restic og rclone. Start her når noe virker ikke å være koblet til.

![Tilpass-modus: kortene kan dras, endres i størrelse og skjules, og de skjulte venter under.](assets/screenshots/overview/customize-dark.png#only-dark)
![Tilpass-modus: kortene kan dras, endres i størrelse og skjules, og de skjulte venter under.](assets/screenshots/overview/customize-light.png#only-light)

## Slik teller BombVault {#counting}

Hvert kort og hodet teller på samme måte: elementene i områdene som er slått på, som fortsatt er installert og som du ikke merket **Ikke sikkerhetskopier**. Et element regnes som beskyttet bare mens en tidsplan når det og det har en sikkerhetskopi. En gammel sikkerhetskopi alene teller ikke, så en container du tok ut av tidsplanen, vises som ubeskyttet til du setter den tilbake eller merker den **Ikke sikkerhetskopier**.

## Første steg {#first-steps}

På en ny installasjon starter oversikten med kortet **Første steg**: **Velg områder**, **Legg til mål** og **Første sikkerhetskopi**. Det blir stående til den første sikkerhetskopien er ferdig. [Kom i gang](getting-started.md#first-run) går gjennom det.
