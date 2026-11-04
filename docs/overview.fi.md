# Yleiskatsaus

Yleiskatsaus on sivupalkin ensimmäinen sivu ja se, jonka tarkistat joka aamu. Se vastaa yhdellä silmäyksellä kahteen kysymykseen: onko kaikki varmuuskopioitu ja tarvitseeko jokin sinua? Se alkaa tilapäällä ja neljällä kortilla, ja lisää kortteja odottaa kohdassa **Mukauta**.

![Yleiskatsaus muutaman päivän jälkeen: tilapää, Vaatii huomiota yhdellä varoituksella, jokaisen osion suojaus, kohteet ja historia.](assets/screenshots/overview/home-dark.png#only-dark)
![Yleiskatsaus muutaman päivän jälkeen: tilapää, Vaatii huomiota yhdellä varoituksella, jokaisen osion suojaus, kohteet ja historia.](assets/screenshots/overview/home-light.png#only-light)

## Tilapää {#status-head}

Pää on sivun yläreunassa ja pysyy siellä aina. Sen rengas näyttää, kuinka paljon on suojattu, esimerkiksi **41 / 43 suojattu**, ja sen vieressä oleva rivi kertoo tilanteen:

| Pää sanoo | Mitä se tarkoittaa |
|---|---|
| **Kaikki on varmuuskopioitu** | Jokaisella päällä olevan osion merkinnällä on ajan tasalla oleva varmuuskopio. |
| **Varmuuskopioitu, huomautuksin** | Kaikki on varmuuskopioitu, ja joistakin ajoista jäi huomautus, esimerkiksi tiedostoista, joita ei voitu lukea. |
| **Kaikki ei ole vielä suojattu** | Joillakin merkinnöillä ei ole ajan tasalla olevaa varmuuskopiota, esimerkiksi koska ne odottavat ensimmäistä. |
| **3 asiaa vaatii huomiota** | Jokin epäonnistui tai odottaa päätöstä. **Vaatii huomiota** -kortti listaa sen. |
| **Varmuuskopiointi käynnissä** | Varmuuskopio on käynnissä. Rengas muuttuu edistymisrenkaaksi, jonka vaiheet ovat **Pysäytys**, **Luku** ja **Käynnistys**, ja täysvarmuuskopio laskee merkintöjään sitä mukaa kuin etenee, esimerkiksi **Varmuuskopiointi käynnissä: 12 / 43**. **Näytä ajo** avaa ajon. |
| **Etäkopiointi käynnissä** | Kopiointi kohteeseen on käynnissä. Se ei näytä prosenttia, koska `restic copy` ei raportoi sellaista. |

Rivin alla näkyy, milloin viimeinen ajo oli ja milloin seuraava suunniteltu alkaa.

## Vaatii huomiota {#needs-you}

Tämä kortti listaa sen, mitä sinun kannattaa katsoa, uusin ensin, ja kertoo **Ei tehtävää.**, kun lista on tyhjä. Jokaisella rivillä on oma ulospääsynsä:

- **Epäonnistunut varmuuskopio** nimeää merkinnän ja syyn. **Näytä virheet** näyttää resticin oman tulosteen, ja **Yritä uudelleen** käynnistää varmuuskopioinnin uudestaan.
- **Etäkopiointi epäonnistui** nimeää osion ja sen, mistä lähtien. **Replikoi nyt** yrittää uudelleen.
- **Poikkeama kohteessa plex** johtaa havaintoon painikkeella **Tarkista**. Kun kriittinen havainto on avoinna, säilytys pitää kyseisen merkinnän vanhat varmuuskopiot.
- **Uudet** merkinnät, jotka BombVault löysi ja joita mikään ei vielä varmuuskopioi, esimerkiksi juuri asentamasi kontti, tarjoavat vaihtoehdot **Aloita varmuuskopiointi** tai **Älä varmuuskopioi**.
- **Varmuuskopio, joka luovutti**, koska sen sovellus pysyi kiireisenä koko odotuksen ajan, katso [Kun sovellus on kiireinen](backups.md#app-busy).
- **Epäonnistunut ZFS-replika**, selkein sanoin, esimerkiksi että palvelimella ei ole zfs-komentoa, ja palvelimen tuloste kohdassa **Näytä**.
- **Varmuuskopiointi on pysähtynyt: avainta ei voi enää avata** ilmestyy, kun tämä palvelin otti toisen palvelimen avaimen käyttöön ja `APP_KEY` muuttui sen jälkeen. Katso [Vianmääritys](troubleshooting.md#backups-stopped-and-say-the-key-can-no-longer-be-opened).
- **Tallenna palautuspakettisi** pysyy, kunnes olet ladannut paketin ja painanut **Olen tallentanut sen turvallisesti**.

## Suojauksen tila {#protection-status}

Jokaisella päällä olevalla osiolla on rivi, jossa on neljä tarkistusta:

| Tarkistus | Se muuttuu vihreäksi, kun | Muita sanoja, jotka saatat nähdä |
|---|---|---|
| **Paikallinen** | viimeinen varmuuskopio on ajan tasalla | ei vielä varmuuskopiota, ei aikataulua, viimeinen ajo epäonnistui, varmuuskopio myöhässä, kaikki merkinnät eivät ole suojattuja, varmuuskopio erääntyy pian |
| **Etä** | viimeinen kopio tiloista ulos on ajan tasalla | ei etäkopiota, kopiointi käynnissä, kopio myöhässä, kopiointi tauolla, ei vielä kopiota, replika epäonnistui |
| **Poistosuoja** | peukalointitesti osoitti, että toinen pää kieltäytyy poistoista | ei mitään, ei mitään ja riski hyväksytty, ei vielä todistettu, testi epäonnistui, ei todistettu, koska sen tarkistus on pois päältä |
| **Koepalautus** | palautus etäkopiosta onnistui | ei koskaan ajettu, käynnissä, epäonnistui, myöhässä, vain käsin, ei mahdollinen tälle osiolle |

Osio on vihreä vain varmennetuilla tosiasioilla. Kopio tällä palvelimella ei ole etäkopio, ja kohde, jonka merkitsit valinnalla **Hyväksy ilman poistosuojaa**, näyttää harmaan renkaan keltaisen pisteen sijaan, joten osion väri seuraa sen muita tarkistuksia.

Ajoitettu osio näyttää **Ei vielä varmuuskopiota** ja pysyy neutraalina, kunnes sen ensimmäisen ajon aika on tullut. Sen jälkeen siinä lukee **Myöhässä**, ja se hälyttää kerran. Osio, jossa ei ole mitään varmuuskopioitavaa, koska mikään aikataulu ei ulotu sen merkintöihin, näyttää **Ei aikataulutettu** eikä ole koskaan myöhässä.

Osion napsautus avaa sen ikkunan, jossa jokainen tarkistus on kirjoitettu auki, mukana ovat sen laskemat merkinnät sekä **Aloita koepalautus** tai **Aja koepalautus uudelleen**. Se näyttää myös **Karsintasääntö asetettu** tai **Ei karsintasääntöä**, ja osion oman kansion säilytyssääntö lasketaan siinä missä sen etäkopioiden säännötkin.

![Kontit-osion suojausikkuna, jossa sen neljä tarkistusta on kirjoitettu auki.](assets/screenshots/overview/protection-window-dark.png#only-dark)
![Kontit-osion suojausikkuna, jossa sen neljä tarkistusta on kirjoitettu auki.](assets/screenshots/overview/protection-window-light.png#only-light)

## Kohteet {#storage-locations}

Jokainen kohde on tässä astia, jossa näkyvät sen täyttöaste, kuinka nopeasti se kasvaa, esimerkiksi **Kasvaa 4,2 Gt/viikko**, ja milloin se olisi täynnä tällä vauhdilla, esimerkiksi **täynnä noin 31 viikon kuluttua**. S3, B2 ja rest-palvelimet eivät raportoi, kuinka paljon tilaa on jäljellä, joten ne näyttävät **Vapaa tila tuntematon** ja vain kasvunsa. Napsautus johtaa kohteen sivulle. Katso [Kohteet](offsite-recovery.md#destinations).

## Historia {#history}

**Historia**-kortissa on kaksi välilehteä. **Ajot** listaa jokaisen ajon tuloksen, keston ja syyn kanssa, ja napsautus avaa sen. **Loki** näyttää jokaisen rivin aikoineen sellaisena kuin BombVault sen kirjoittaa, ja sen voi rajata yhteen päivään. Rivi muuttuu oranssiksi, kun varmuuskopio onnistui mutta jätti jotakin pois, ja rivi nimeää sen, minkä se jätti pois.

## Lisää kortteja kohdassa Mukauta {#customize}

**Mukauta** sivun yläreunassa antaa järjestää kortit. Siirrä korttia sen palkista vetämällä, muuta sen kokoa alakulmassa olevasta pisteestä tai piilota se. Piilotetut kortit odottavat kohdassa **Piilotetut kortit**, ja **Näytä** tuo yhden takaisin. Kun järjestät niitä, kortit keinuvat hieman, ja **Valmis** lopettaa sen. Asettelu tallentuu tähän selaimeen.

Nämä kortit ovat aluksi piilossa:

- **Poikkeamat**: avoimet havainnot, katso [Poikkeamat](anomalies.md).
- **Ei varmuuskopioitu**: kaikki tällä palvelimella, jota mikään ajastettu varmuuskopio ei kata, kullekin syy, esimerkiksi **ei koskaan lisätty**, **ei sisälly aikatauluun** tai **kaikki sen osion kohteet on kytketty pois päältä**. Se lukee kontit suoraan Dockerista, joten se löytää myös sellaisen, jota BombVault ei ole koskaan nähnyt.
- **Kalenteri**: viimeiset 90 päivää, yksi ruutu päivää kohti. Päivän napsautus näyttää sen rivit lokissa kohdassa **Historia**.
- **Tallennustila**: kuinka paljon kukin osio vie ja kuinka hyvin sen data deduplikoituu.
- **Yleiskatsaus**: vanhin tilannevedos kaikissa osioissa, milloin seuraava varmuuskopio ajetaan ja miten viimeinen meni.
- **Tilastot**: kuinka monta konttia ja virtuaalikonetta on, kuinka monta virhettä ja kuinka monta aikataulua on aktiivisena tai tauolla.
- **Isäntäintegraatio**: mihin BombVault ylettyy ja mitä se tarvitsee isännältä, kuten Docker-soketti, libvirt, restic ja rclone. Aloita tästä, kun jokin tuntuu olevan kytkemättä.

![Mukauta-tila: kortteja voi vetää, niiden kokoa voi muuttaa ja ne voi piilottaa, ja piilotetut odottavat alla.](assets/screenshots/overview/customize-dark.png#only-dark)
![Mukauta-tila: kortteja voi vetää, niiden kokoa voi muuttaa ja ne voi piilottaa, ja piilotetut odottavat alla.](assets/screenshots/overview/customize-light.png#only-light)

## Miten BombVault laskee {#counting}

Jokainen kortti ja pää laskevat samalla tavalla: päällä olevien osioiden merkinnät, jotka ovat vielä asennettuina ja joita et merkinnyt kohtaan **Älä varmuuskopioi**. Merkintä lasketaan suojatuksi vain, kun aikataulu ylettyy siihen ja sillä on varmuuskopio. Pelkkä vanha varmuuskopio ei riitä, joten kontti, jonka otit pois aikataulusta, näkyy suojaamattomana, kunnes palautat sen tai merkitset sen kohtaan **Älä varmuuskopioi**.

## Ensimmäiset vaiheet {#first-steps}

Uudessa asennuksessa yleiskatsaus alkaa **Ensimmäiset vaiheet** -kortilla: **Valitse osiot**, **Lisää kohde** ja **Ensimmäinen varmuuskopio**. Se pysyy, kunnes ensimmäinen varmuuskopio on valmis. [Aloitus](getting-started.md#first-run) opastaa sen läpi.
