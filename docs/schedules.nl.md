# Planningen

**Instellingen, Planning** bevat elke planning: de volledige back-up die alles dekt, een eigen planning voor een onderdeel, de off-site-kopieën, welke items meedoen en in welke volgorde. Een enkel item kan op zijn pagina een eigen planning krijgen. Deze pagina legt uit hoe ze samenhangen en loopt een gebruikelijke opstelling door.

## Planningen lezen als zinnen {#plans}

Elke planning is een knop die als een zin leest, zoals *dagelijks om 03:00* of *elke 2 dagen om 01:30*. Een klik opent hem met vijf modi:

| Modus | Wat je instelt |
|---|---|
| **Uit** | Op deze planning loopt niets. |
| **Dagelijks** | **Tijd** |
| **Wekelijks** | **Dagen** en **Tijd** |
| **Elke N dagen** | **Elke** zoveel **dagen**, en de **Tijd**. Het telt vanaf de laatste geplande run, dus een back-up van één item met de hand schuift de planning niet op. |
| **Cron** | Een **Expressie** van vijf velden, zoals `0 */6 * * *` voor elke 6 uur. **Voorbeelden** biedt er een paar aan, en het venster toont de volgende runs. |

Alle tijden zijn in de tijdzone van de server. Op Unraid is dat die van het systeem zelf. Elders stel je `TZ` in, anders draait elke planning in UTC, zie [Omgevingsvariabelen](configuration.md#environment-variables).

## Week {#week}

De kaart **Week** tekent elke geplande run van maandag tot zondag, één regel per dag: back-ups, off-site-kopieën en controles. Wijs op een streepje om te lezen wat daar start. Waar drie planningen binnen twee uur starten, is het stuk gemarkeerd, zodat je ze kunt spreiden.

![Instellingen, Planning: de band Week bovenaan, dan de volledige back-up en de planningen per onderdeel.](assets/screenshots/schedule/week-dark.png#only-dark)
![Instellingen, Planning: de band Week bovenaan, dan de volledige back-up en de planningen per onderdeel.](assets/screenshots/schedule/week-light.png#only-light)

## Volledige back-up {#full-backup}

De **Volledige back-up** is de standaardplanning voor alles. **Wanneer back-ups draaien** stelt haar in: één ronde die elk onderdeel om beurt back-upt, containers, VM's, flash, mappen, ZFS-datasets en de zelf-back-up als laatste. De regel eronder zegt welke onderdelen erin zitten. **Uit** schakelt haar uit, en dan draait alleen wat een eigen planning heeft.

- **Gemiste back-ups na de start inhalen**: was de server uit op een geplande tijd, dan voert BombVault die back-up ongeveer twee minuten na zijn start uit. Een eigen planning van een item en een off-site-planning wachten op hun volgende geplande tijd, net als een planning die nog nooit is geslaagd.
- **Opdrachten ervoor en erna**: **Opdracht vóór de volledige back-up** draait op de server met `sh -c` voordat de ronde begint, bijvoorbeeld een script dat een dienst stopt. Mislukt die, dan draait de volledige back-up toch. **Opdracht na de volledige back-up** draait één keer nadat elk onderdeel is geprobeerd, ook als er een mislukte, wat past bij een ping die bevestigt dat de hele server beschermd is. Opdrachten voor één container staan op zijn pagina.

**Nu alles back-uppen** op de pagina Back-ups start dezelfde ronde met de hand, over elk ingeschakeld onderdeel.

## Planningen per onderdeel {#plans-per-section}

Een onderdeel kan de volledige back-up verlaten en op een eigen tijd draaien. In **Planningen per onderdeel** voegt **Uitzondering** een onderdeel met een eigen planning toe, en **Uitzondering verwijderen** stuurt het terug. De regel in de kaart zegt wat de andere doen, zoals **Alle andere onderdelen draaien in de volledige back-up.**

Een onderdeel met een eigen planning wordt in de volledige back-up niet nog eens geback-upt, dus er wordt niets dubbel geback-upt. Hetzelfde geldt voor een enkel item met een eigen planning.

**Off-site-kopieën per onderdeel** werkt op dezelfde manier voor kopieën. Standaard kopieert elk onderdeel direct na elke back-up naar zijn off-site-bestemmingen. **Uitzondering** geeft één onderdeel een eigen kopieerplanning, bijvoorbeeld zodat de VM's alleen op zondagnacht naar Backblaze B2 kopiëren. De eigen kaart **Kopiëren** van een bestemming kan ook **Wanneer kopiëren** instellen, zie [Off-site-kopieën](offsite-recovery.md#off-site-copies).

## Geplande items {#scheduled-entries}

**Geplande items** zegt per onderdeel hoeveel items meedoen, zoals **38 van 41 gepland**. **Items** opent de lijst met een schakelaar **In de planning** per item. Een item dat is uitgeschakeld, wordt alleen geback-upt als je het met de hand start. Het telt als onbeschermd tenzij je het markeert met **Niet back-uppen**.

## Volgorde {#order}

**Volgorde** bepaalt in welke volgorde geplande runs en de volledige back-up containers en VM's back-uppen. Sleep een item op zijn plek of gebruik de pijlen, en elke verplaatsing wordt meteen opgeslagen. Items die je ongeordend laat, worden geback-upt met de langst achterstallige eerst. Een enkele back-up met de hand blijft onaangetast.

**Herstarten na back-up** in dezelfde kaart betreft de containers die **Andere containers stoppen** tijdens een back-up heeft stilgelegd. Ze starten weer in hun afhankelijkheidsvolgorde, en met **Wachten tot afhankelijkheden gezond zijn voordat de volgende start** moet elke container healthy of running melden voordat de volgende start. **Gezondheidstime-out per container (seconden)** begrenst het wachten, standaard 120, in een bereik van 5 tot 3600.

## Snapshot in plaats van stoppen {#snapshot-instead-of-stop}

**Containers op ZFS alleen voor een snapshot stoppen** staat standaard aan. Een container waarvan de data op ZFS ligt, wordt alleen gestopt terwijl BombVault een snapshot van die data maakt, meestal een paar seconden, en de back-up wordt dan uit de snapshot gelezen. Het heeft de SSH-verbinding met de host nodig die ook ZFS-datasets gebruiken. Een container waarvan een deel van de data elders ligt, wordt zoals voorheen voor de hele back-up gestopt, en zijn run zegt waarom, terwijl een container zonder data op ZFS zonder opmerking wordt gestopt. Het werkt op een Unraid-gebruikersshare die op één ZFS-pool ligt. Op TrueNAS heeft de app ook de capability `SYS_CHROOT` nodig, zie [TrueNAS SCALE](getting-started.md#truenas). Een enkele container kan op zijn eigen pagina anders worden ingesteld.

De back-up komt onder dezelfde paden en tags terecht als een die met de container gestopt is gemaakt, dus herstellen en controles behandelen beide gelijk. De opdracht vóór de back-up en de databasedump draaien nog steeds eerst, de andere containers die moeten stoppen gaan eronder en komen erbij terug, en de opdracht na de back-up draait zodra de container weer draait. Een update na de back-up wacht nog steeds tot de back-up klaar is. Zijn runs tonen **Momentopname** en hoeveel seconden de container stillag. Stopt BombVault midden in een containerback-up, dan start het de containers die het stillegde de volgende keer dat het start weer.

## Updates na de back-up {#updates}

- **Containers bijwerken na een geslaagde back-up** haalt pas een nieuwere image op nadat de back-up ervoor klaar is, zodat er altijd eerst een vers herstelpunt bestaat. Het staat standaard uit, en een enkele container kan op zijn pagina anders worden ingesteld onder **Bijwerken na een geslaagde back-up**. Alleen wat je zelf hebt ingesteld, gaat mee naar de nieuwe image: variabelen, labels, het commando, de gebruiker en de healthcheck die nog zo luiden als in de oude image, volgen de nieuwe. Kan de nieuwe image niet worden aangemaakt of gestart, dan gaat de container terug naar de image waarop hij draaide, staat de update als mislukt, en probeert de volgende back-up het opnieuw. Een container die daardoor niet meer draait, wordt genoemd in de run van de back-up en in een melding.
- **Oude image verwijderen na een update** verwijdert de image die de container achterliet. Een basisimage die andere containers delen, wordt nooit verwijderd.
- **Unraids updatestatus vernieuwen na het bijwerken van een container** wist de updatebanner van Unraid voor die container.

Een bericht per bijgewerkte container stel je in onder **Instellingen, Meldingen** met **Melden bij containerupdate**.

## Rust voor de back-up {#idle}

De kaart **Rust voor de back-up** bepaalt wanneer een app als rustig telt voor een container waarvan de geplande back-up erop wacht: **CPU onder (% van een kern)**, **Verkeer onder (Mbit/s)** en **Minstens (min)**. Een mediaserver telt als rustig als hij niet streamt, en voor een container op het hostnetwerk telt alleen de CPU. Het wachten zelf stel je per container in, zie [Als de app bezig is](backups.md#app-busy).

## Een eigen planning voor één item {#own-plan}

Elk item heeft **Eigen planning** in de kaart **Planning en bestemming** op zijn tabblad Instellingen. Met een planning daar draait het item op zijn eigen tijden en verlaat het de volledige back-up en de planning van zijn onderdeel. Zonder volgt het die.

## Voorbeeld: drie planningen die bij elkaar passen {#example}

Een gebruikelijke Unraid-machine: de meeste containers kunnen 's nachts worden geback-upt, de VM's zijn groot en veranderen langzaam, en Immich krijgt de hele dag nieuwe foto's.

1. **Volledige back-up om 03:00.** Open **Instellingen, Planning**. Zet in **Volledige back-up** **Wanneer back-ups draaien** op **Dagelijks** om `03:00`. Schakel **Gemiste back-ups na de start inhalen** in, zodat een nacht met de server uit wordt ingehaald.
2. **VM's eens per week.** Druk in **Planningen per onderdeel** op **Uitzondering**, kies **VM's** en zet zijn planning op **Wekelijks**, **Dagen** zondag, **Tijd** `04:00`. De kaart zegt nu dat alle andere onderdelen in de volledige back-up draaien.
3. **VM's kopiëren zondag ook off-site.** Druk in **Off-site-kopieën per onderdeel** op **Uitzondering**, kies **VM's** en stel **Wekelijks** in, zondag om `06:00`, nadat hun back-up tijd heeft gehad om klaar te komen.
4. **Immich elke 6 uur.** Open **Back-ups**, dan `immich`, dan zijn tabblad **Instellingen**. Zet **Eigen planning** op **Cron** met `0 */6 * * *`.

Wat er nu draait:

| Wanneer | Wat |
|---|---|
| Elke dag om 03:00 | Containers behalve `immich`, flash, mappen, ZFS-datasets en de zelf-back-up, het ene onderdeel na het andere |
| Zondag om 04:00 | VM's |
| Zondag om 06:00 | De off-site-kopie van de VM's |
| 00:00, 06:00, 12:00 en 18:00 | `immich` op zichzelf |

De kaart **Week** toont de lijn van 03:00 op elke dag, de streepjes van 04:00 en 06:00 op zondag, en vier streepjes per dag voor `immich`. `immich` wordt om 03:00 niet een tweede keer geback-upt, omdat een item met een eigen planning de volledige back-up verlaat. Kopieën van de andere onderdelen draaien nog steeds direct na elke back-up.

Verwijder je later de eigen planning van `immich`, dan doet hij weer mee met de run van 03:00, met zijn onderdeel.
