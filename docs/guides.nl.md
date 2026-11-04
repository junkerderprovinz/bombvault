# Uitgewerkte voorbeelden

Elk voorbeeld loopt één echte klus van begin tot eind door, met namen en getallen ingevuld. Ze staan op de pagina die de functie uitlegt, zodat de details er direct naast staan.

## Aan de slag {#start}

- [Je eerste containerback-up, van begin tot eind](getting-started.md#example-first-backup): van de installatie tot de herstelkit.

## Opslag en off-site {#storage}

- [Een off-site-kopie op Backblaze B2](offsite-recovery.md#example-s3): een bucket, een sleutel voor die bucket en de eerste kopie. Cloudflare R2 werkt op dezelfde manier.
- [Een rest-server die je zelf draait](offsite-recovery.md#example-rest-server): een off-site-kopie die verwijderen weigert, met de regel `docker run` en de sabotagetest.
- [Wat Gebalanceerd bewaart](offsite-recovery.md#example-retention): bewaarbeleid over 400 dagen nachtelijke back-ups, back-up voor back-up.
- [Containers verhuizen naar een NAS](offsite-recovery.md#example-own-folder): de eigen map van een onderdeel verandert van plek, en wat er gebeurt met de al gemaakte back-ups.

## Back-ups en planningen {#backups}

- [Drie planningen die bij elkaar passen](schedules.md#example): een volledige back-up 's nachts, VM's eens per week en een eigen planning voor één item om de 6 uur.
- [Plex wacht zolang iemand kijkt](backups.md#example-plex): een geplande back-up die geen film onderbreekt.
- [Een VM met twee Unraid-snapshots](vm-backup-ssh-setup.md#example-unraid-snapshots): een live back-up die de snapshots bewaart, en een herstel naast het origineel.
- [Een replica naar een tweede Unraid-machine](zfs-datasets.md#example-replica): een ZFS-kopie die je op elk moment kunt lezen, en drie manieren om er één bestand uit te halen.

## Dingen terughalen {#restore}

- [Eén bestand terughalen](restore.md#example-single-file): een enkel bestand uit de containerback-up van afgelopen nacht.
- [Eén bestand uit een ZFS-back-up](zfs-datasets.md#example-zfs-file): hetzelfde voor een dataset.
- [Een nieuwe server vanuit de herstelkit](offsite-recovery.md#example-emergency): een kapotte server komt terug op een nieuwe machine, zonder zijn oude `APP_KEY` in de template.

## Op de hoogte blijven {#staying-informed}

- [Gotify op je telefoon](notifications.md#example-gotify): mislukkingen gaan over, successen komen stil binnen.
- [ntfy als webhook](notifications.md#example-ntfy): berichten zonder account.

## Meerdere servers {#several-servers}

- [Twee Unraid-machines koppelen](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end): twee servers die elkaars kopieën bewaren, waarbij geen van beide de back-ups van de ander kan verwijderen.
- [Een kaart en een melding in Home Assistant](api.md#home-assistant-example): BombVault op een dashboard, met een bericht als een back-up mislukt.
