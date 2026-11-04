# Anomalieën

Ransomware en een kapotte app lijken in een back-up op elkaar: ineens is het grootste deel van de data nieuw, of het grootste deel is weg. BombVault vergelijkt elke back-up met de eigen geschiedenis van dat item en meldt een bevinding als er iets niet klopt. Zolang een bevinding over verloren data openstaat, houdt BombVault ook de oude back-ups van het item vast, zodat de laatste goede niet kan verouderen. Het item **Anomalieën** in de zijbalk is zichtbaar zolang de detectie aanstaat.

![De pagina Anomalieën met één openstaande kritieke bevinding.](assets/screenshots/anomalies/page-dark.png#only-dark)
![De pagina Anomalieën met één openstaande kritieke bevinding.](assets/screenshots/anomalies/page-light.png#only-light)

## De pagina {#page}

Met de tegels **Kritiek**, **Waarschuwingen** en **Opmerkingen** kies je een ernst, en met **Open** en **Gesloten** boven de lijst wissel je tussen wat openstaat en wat de afgelopen 30 dagen is gesloten. Een kritieke bevinding staat op hetzelfde vlak als de andere. Haar ernst zie je aan de badge en de gekleurde rand. Een bevinding biedt eerst de oplossing aan, en in de details staat de waarde als een curve over het gebruikelijke bereik.

Staat er niets open, dan toont **Leervoortgang** per item een ring van tien delen, één deel voor elke back-up waarvan BombVault heeft geleerd. **Bewaking** opent de lijst met items en hun eigen instellingen.

## Wat wordt gecontroleerd {#checks}

Elke back-up van een container, VM, mappenset, databasedump, de flash en de zelf-back-up wordt vergeleken met de eigen geschiedenis van het item. De controles kijken naar:

- de nieuwe data die een run heeft opgeslagen, tegenover de grootste gebruikelijke hoeveelheden van recente back-ups en tegenover de gebruikelijke snelheid per uur,
- een back-up die het grootste deel van de data opnieuw heeft opgeslagen, inclusief bestanden die een andere naam kregen en herschreven zijn,
- de bronomvang en het aantal bestanden die restic voor elk item en elke databasedump meldt,
- de eigen back-uptijd van restic,
- reeksen mislukkingen en mislukkingen die af en toe optreden,
- herstelcontroles die niet meer slagen,
- de vrije ruimte van lokale, SFTP- en rclone-repository's, berekend uit de groei van het repository.

Een item leert van zijn eerste 10 back-ups. Een bijna lege bron, een herschrijving van het grootste deel van de data en mislukkingen worden vanaf het begin gecontroleerd.

Een bevinding over de omvang of het aantal bestanden van een bron krijgt de datum van de eerste back-up waarin dat zichtbaar was. **Vergelijken met de back-up ervoor** toont de mappen waar bestanden verdwenen, erbij kwamen of veranderden, met een opmerking als bijna alles in een zoekindex, een cache of miniaturen zit die de app zelf opnieuw opbouwt.

ZFS-items worden dataset voor dataset bewaakt, zie [ZFS-datasets](zfs-datasets.md#anomalies). Een assistent kan de openstaande bevindingen lezen via de [MCP-server](mcp.md#tools).

De controles hebben grenzen. S3-, B2- en REST-repository's hebben geen cijfer voor vrije ruimte, op de Unraid-share van de gebruiker is de vrije ruimte die van de hele array, en back-ups van voor restic 0.17 hebben geen omvanggeschiedenis.

## Een bevinding sluiten {#closing}

- **Waarschuwingen** sluiten vanzelf zodra de oorzaak weg is.
- **Kritieke** bevindingen over verloren data en een vollopende schijf blijven staan tot je ingrijpt.
- **Bevestigen** markeert een bevinding als gezien en sluit haar. Ze wordt pas opnieuw gemeld nadat de oorzaak een keer is verdwenen.
- **Als verwacht markeren** zegt dat de verandering van jou kwam. Het nieuwe niveau wordt na 10 back-ups normaal, maar de controle op een bijna lege bron wordt nooit uitgeschakeld.
- Een gewijzigde selectie, zoals een map die je aan- of uitvinkte, laat de geschiedenis van het item vanzelf opnieuw beginnen.

Een melding gaat één keer per episode de deur uit, en mislukkingen en herstelcontroles die al een melding sturen, worden niet dubbel gemeld.

## Oude back-ups blijven intussen bewaard {#retention-hold}

Zolang een bron bijna leeg is, sterk is gekrompen of het grootste deel van zijn data opnieuw is opgeslagen, laten bewaarbeleid en opschonen de oude back-ups van dat item met rust, en de bevinding linkt naar de laatste goede back-up, die het herstelvenster dan alvast selecteert. Voordat je zo'n bevinding sluit, meldt BombVault dat de volgende opruiming oude back-ups weer volgens de bewaarregels verwijdert. Ontbreekt er dus data of is ze versleuteld, herstel dan eerst en sluit de bevinding daarna.

## Instellingen {#settings}

De kaart **Anomalieën** onder **Instellingen, Verificatie** bevat de algemene instellingen. Elke instelling wordt opgeslagen zodra je haar wijzigt, en de drie onder de schakelaar zijn verborgen zolang de detectie uitstaat.

| Instelling | Standaard | Wat het doet |
|---|---|---|
| **Anomalieën detecteren** | Aan | Vergelijkt elke back-up met de eigen geschiedenis van het item. Staat het uit, dan wordt niets nieuws gecontroleerd en verdwijnt het item **Anomalieën** uit de zijbalk. De kaart linkt nog steeds naar eerdere bevindingen. |
| **Gevoeligheid** | Gebalanceerd | Streng meldt kleinere veranderingen, Soepel alleen grote. |
| **Een melding sturen voor** | Alleen kritieke bevindingen | De laagste ernst die een bericht stuurt via de kanalen die je instelt onder [Meldingen](notifications.md). Herhaalde mislukte back-ups en dumps en mislukte geplande herstelcontroles sturen al hun eigen bericht en worden niet dubbel gestuurd. |
| **Oude back-ups bewaren als een bron sterk krimpt of wordt herschreven** | Aan | Zolang een item een openstaande bevinding heeft voor een bijna lege bron, een sterke krimp of het grootste deel van zijn data opnieuw opgeslagen, laten bewaarbeleid en opschonen de oude back-ups van dat item met rust. Bevestig de bevinding of markeer haar als verwacht om ze los te laten. |

Elk item kan een eigen gevoeligheid en een eigen minimum voor meldingen gebruiken. Stel ze in op het tabblad **Verificatie** van het item, of op de pagina **Anomalieën** onder **Bewaking**. Voor een ZFS-item gelden ze voor elke dataset van de boom.
