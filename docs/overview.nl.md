# Overzicht

Het overzicht is de eerste pagina in de zijbalk en die om elke ochtend te bekijken. Het beantwoordt in één oogopslag twee vragen: is alles geback-upt, en heeft iets je nodig? Het begint met een statuskop en vier kaarten, en meer kaarten wachten onder **Aanpassen**.

![Het overzicht na een paar dagen: de statuskop, Aandacht nodig met één waarschuwing, de bescherming van elk onderdeel, de bestemmingen en de geschiedenis.](assets/screenshots/overview/home-dark.png#only-dark)
![Het overzicht na een paar dagen: de statuskop, Aandacht nodig met één waarschuwing, de bescherming van elk onderdeel, de bestemmingen en de geschiedenis.](assets/screenshots/overview/home-light.png#only-light)

## De statuskop {#status-head}

De kop staat bovenaan de pagina en blijft daar altijd. Zijn ring toont hoeveel er beschermd is, zoals **41 van 43 beschermd**, en de regel ernaast zegt hoe het ervoor staat:

| De kop zegt | Wat het betekent |
|---|---|
| **Alles is geback-upt** | Elk item van elk ingeschakeld onderdeel heeft een actuele back-up. |
| **Back-up gemaakt, met opmerkingen** | Alles is geback-upt, en sommige runs lieten een opmerking achter, zoals bestanden die ze niet konden lezen. |
| **Nog niet alles is beschermd** | Sommige items hebben geen actuele back-up, bijvoorbeeld omdat ze op hun eerste wachten. |
| **3 dingen hebben je aandacht nodig** | Er is iets mislukt of wacht op een beslissing. De kaart **Aandacht nodig** toont het. |
| **Back-up wordt gemaakt** | Er loopt een back-up. De ring wordt een voortgangsring met de stations **Stoppen**, **Lezen** en **Starten**, en een volledige back-up telt zijn items terwijl hij loopt, zoals **Back-up wordt gemaakt: 12 van 43**. **Run bekijken** opent de run. |
| **Off-site-kopie loopt** | Er loopt een kopie naar een bestemming. Ze toont geen percentage, omdat `restic copy` er geen meldt. |

Daaronder staat wanneer de laatste run was en wanneer de volgende geplande start.

## Aandacht nodig {#needs-you}

Deze kaart toont wat je moet bekijken, nieuwste eerst, en zegt **Niets te doen.** als de lijst leeg is. Elke rij heeft zijn eigen uitweg:

- **Een mislukte back-up** noemt het item en de reden. **Fouten bekijken** toont de eigen uitvoer van restic, en **Opnieuw proberen** start de back-up nog eens.
- **Off-site-kopie mislukt** noemt het onderdeel en sinds wanneer. **Nu repliceren** probeert het opnieuw.
- **Anomalie bij plex** leidt met **Controleren** naar de bevinding. Zolang een kritieke bevinding openstaat, bewaart het bewaarbeleid de oude back-ups van dat item.
- **Nieuwe** items die BombVault vond en die nog niet worden geback-upt, zoals een container die je net hebt geïnstalleerd, bieden **Beginnen met back-uppen** of **Niet back-uppen** aan.
- **Een back-up die het opgaf** omdat zijn app de hele wachttijd bezig bleef, zie [Als de app bezig is](backups.md#app-busy).
- **Een mislukte ZFS-replica**, in gewone woorden zoals dat de server geen zfs-opdracht heeft, met de uitvoer van de server onder **Details**.
- **Back-ups staan stil: de sleutel is niet meer te openen** verschijnt wanneer deze server de sleutel van een andere server overnam en `APP_KEY` daarna veranderde. Zie [Problemen oplossen](troubleshooting.md#backups-stopped-and-say-the-key-can-no-longer-be-opened).
- **Bewaar je herstelkit** blijft staan tot je de kit hebt gedownload en op **Ik heb hem veilig bewaard** hebt gedrukt.

## Beschermingsstatus {#protection-status}

Elk ingeschakeld onderdeel heeft een rij met vier controles:

| Controle | Ze wordt groen wanneer | Andere woorden die je kunt zien |
|---|---|---|
| **Lokaal** | de laatste back-up actueel is | nog geen back-up, geen planning, laatste run mislukt, back-up achterstallig, niet elk item is beschermd, back-up binnenkort aan de beurt |
| **Off-site** | de laatste kopie buiten de locatie actueel is | geen off-site-kopie, kopie loopt, kopie achterstallig, kopie gepauzeerd, nog geen kopie, replica mislukt |
| **Bescherming tegen verwijderen** | een sabotagetest bewees dat de andere kant verwijderen weigert | geen, geen en risico geaccepteerd, nog niet bewezen, test mislukt, niet bewezen omdat de controle uitstaat |
| **Testherstel** | een herstel vanaf de off-site-kopie slaagde | nooit uitgevoerd, loopt, mislukt, achterstallig, alleen handmatig, niet mogelijk voor dit onderdeel |

Een onderdeel is alleen groen op geverifieerde feiten. Een kopie op deze server telt niet als off-site, en een bestemming die je markeerde met **Accepteren zonder bescherming tegen verwijderen** toont een grijze ring in plaats van een amberkleurige stip, zodat de kleur van het onderdeel zijn andere controles volgt.

Een klik op een onderdeel opent zijn venster met elke controle uitgeschreven, de items die het telt en **Testherstel starten** of **Testherstel opnieuw uitvoeren**.

![Het beschermingsvenster van het onderdeel Containers met zijn vier controles uitgeschreven.](assets/screenshots/overview/protection-window-dark.png#only-dark)
![Het beschermingsvenster van het onderdeel Containers met zijn vier controles uitgeschreven.](assets/screenshots/overview/protection-window-light.png#only-light)

## Bestemmingen {#storage-locations}

Elke bestemming staat hier als een vat met zijn vulniveau, hoe snel hij groeit, zoals **Groeit 4,2 GB/week**, en wanneer hij bij die snelheid vol zou zijn, zoals **vol over ongeveer 31 weken**. S3, B2 en rest-servers melden niet hoeveel ruimte er nog is, dus ze tonen **Vrije ruimte onbekend** en alleen hun groei. Een klik leidt naar de pagina van de bestemming. Zie [Bestemmingen](offsite-recovery.md#destinations).

## Geschiedenis {#history}

**Geschiedenis** heeft twee tabbladen. **Runs** toont elke run met zijn resultaat, duur en reden, en een klik opent hem. **Logboek** toont elke regel met zijn tijd, zoals BombVault hem schrijft, en kan worden beperkt tot één dag. Een regel wordt oranje wanneer een back-up doorging maar iets oversloeg, en de regel noemt wat hij oversloeg.

## Meer kaarten onder Aanpassen {#customize}

Met **Aanpassen** bovenaan de pagina rangschik je de kaarten. Sleep een kaart aan zijn balk om hem te verplaatsen, verander zijn grootte bij de stip in zijn hoek onderaan, of verberg hem. Verborgen kaarten wachten onder **Verborgen kaarten**, en **Weergeven** haalt er een terug. Terwijl je ze rangschikt, wiebelen de kaarten een beetje, en **Klaar** beëindigt dat. De indeling wordt in deze browser opgeslagen.

Deze kaarten beginnen verborgen:

- **Anomalieën**: de openstaande bevindingen, zie [Anomalieën](anomalies.md).
- **Zonder back-up**: alles op deze server wat geen geplande back-up dekt, elk met zijn reden, zoals **nooit toegevoegd**, **niet opgenomen in de planning** of **alle bestemmingen van zijn onderdeel staan uit**. Ze leest containers zelf uit Docker, dus ze vindt ook een container die BombVault nooit heeft gezien.
- **Kalender**: de laatste 90 dagen, één vierkantje per dag. Een klik op een dag toont zijn regels in het logboek onder **Geschiedenis**.
- **Opslag**: hoeveel ruimte elk onderdeel inneemt en hoe goed zijn data dedupliceert.
- **Overzicht**: de oudste snapshot over alle onderdelen, wanneer de volgende back-up loopt en hoe de laatste ging.
- **Statistieken**: hoeveel containers en VM's er zijn, hoeveel fouten, en hoeveel planningen actief of gepauzeerd zijn.
- **Host-integratie**: wat BombVault op de host bereikt en nodig heeft, zoals de Docker-socket, libvirt, restic en rclone. Begin hier als er iets niet goed aangesloten lijkt.

![Aanpassen: de kaarten kunnen worden gesleept, vergroot en verborgen, en de verborgen wachten eronder.](assets/screenshots/overview/customize-dark.png#only-dark)
![Aanpassen: de kaarten kunnen worden gesleept, vergroot en verborgen, en de verborgen wachten eronder.](assets/screenshots/overview/customize-light.png#only-light)

## Hoe BombVault telt {#counting}

Elke kaart en de kop tellen op dezelfde manier: de items van de ingeschakelde onderdelen die nog geïnstalleerd zijn en die je niet markeerde met **Niet back-uppen**. Een item telt alleen als beschermd zolang een planning het bereikt en het een back-up heeft. Een oude back-up alleen telt niet mee, dus een container die je uit de planning haalde, staat als onbeschermd tot je hem terugzet of markeert met **Niet back-uppen**.

## Eerste stappen {#first-steps}

Op een nieuwe installatie begint het overzicht met de kaart **Eerste stappen**: **Kies onderdelen**, **Bestemming toevoegen** en **Eerste back-up**. Ze blijft staan tot de eerste back-up klaar is. [Aan de slag](getting-started.md#first-run) loopt het langs.
