# Poikkeamat

Kiristysohjelma ja rikkinäinen sovellus näyttävät varmuuskopiossa samalta: yhtäkkiä suurin osa tiedoista on uutta tai suurin osa on kadonnut. BombVault vertaa jokaista varmuuskopiota kyseisen merkinnän omaan historiaan ja tekee havainnon, kun jokin näyttää väärältä. Kun havainto kadonneista tiedoista on avoinna, BombVault säilyttää myös merkinnän vanhat varmuuskopiot, joten viimeinen hyvä ei pääse vanhenemaan. Sivupalkin **Poikkeamat**-kohta näkyy, kun tunnistus on päällä.

![Poikkeamat-sivu, jolla on yksi avoin kriittinen havainto.](assets/screenshots/anomalies/page-dark.png#only-dark)
![Poikkeamat-sivu, jolla on yksi avoin kriittinen havainto.](assets/screenshots/anomalies/page-light.png#only-light)

## Sivu {#page}

**Kriittinen**-, **Varoitukset**- ja **Huomiot**-ruudut valitsevat vakavuuden. Listan yläpuolella olevat **Avoin** ja **Suljettu** vaihtavat näkymän avoimiin havaintoihin ja viimeisten 30 päivän aikana suljettuihin. Kriittinen havainto on samalla pinnalla kuin muut, ja sen vakavuus näkyy merkistä ja värillisestä reunasta. Havainto tarjoaa korjauksen ensimmäisenä, ja sen tiedoissa arvo näkyy käyränä tavanomaisella vaihteluvälillä.

Kun mitään ei ole avoinna, **Oppimisen edistyminen** näyttää jokaiselle merkinnälle kymmenen osan renkaan, yhden osan jokaista varmuuskopiota kohti, josta BombVault on oppinut. **Valvonta** avaa listan merkinnöistä ja niiden omista asetuksista.

## Mitä se tarkistaa {#checks}

Jokaista kontin, virtuaalikoneen, kansiojoukon, tietokantavedoksen, flash-aseman ja itsevarmuuskopion varmuuskopiota verrataan merkinnän omaan historiaan. Tarkistukset katsovat seuraavia asioita:

- ajon tallentamaa uutta dataa verrattuna viimeaikaisten varmuuskopioiden suurimpiin tavallisiin määriin ja tavanomaiseen tuntinopeuteen,
- varmuuskopiota, joka tallensi suurimman osan datasta uudelleen, mukaan lukien uudelleennimetyt ja uudelleenkirjoitetut tiedostot,
- lähteen koko ja tiedostomäärä, jotka restic raportoi jokaiselle merkinnälle ja jokaiselle tietokantavedokselle,
- resticin oma varmuuskopiointiaika,
- epäonnistumisjaksot ja ajoittaiset epäonnistumiset,
- palautustarkistukset, jotka lakkasivat läpäisemästä,
- paikallisten, SFTP- ja rclone-repojen vapaa tila, ennustettuna repon kasvusta.

Merkintä oppii ensimmäisistä 10 varmuuskopiostaan. Lähes tyhjä lähde, suurimman osan datasta uudelleenkirjoitus ja epäonnistumiset tarkistetaan alusta asti.

Lähteen kokoa tai tiedostomäärää koskeva havainto päivätään ensimmäiseen varmuuskopioon, jossa muutos näkyi. **Vertaa edelliseen varmuuskopioon** listaa kansiot, joissa tiedostoja katosi, tuli lisää tai muuttui. Mukana on huomautus, jos lähes kaikki on hakuindeksissä, välimuistissa tai pikkukuvissa, jotka sovellus rakentaa itse uudelleen.

ZFS-merkintöjä valvotaan tietojoukko kerrallaan, katso [ZFS-tietojoukot](zfs-datasets.md#anomalies). Avustaja voi lukea avoimet havainnot [MCP-palvelimen](mcp.md#tools) kautta.

Tarkistuksilla on rajansa. S3-, B2- ja REST-repoilla ei ole vapaan tilan lukua, Unraidin käyttäjäjaossa vapaa tila on koko arrayn vapaa tila, ja ennen restic 0.17:ää tehdyillä varmuuskopioilla ei ole kokohistoriaa.

## Havainnon sulkeminen {#closing}

- **Varoitukset** sulkeutuvat itsestään, kun syy on poistunut.
- **Kriittiset** havainnot kadonneesta datasta ja täyttyvästä levystä pysyvät, kunnes toimit.
- **Kuittaa** merkitsee havainnon nähdyksi ja sulkee sen. Sitä ei nosteta uudelleen, ennen kuin sen syy on kerran poistunut.
- **Merkitse odotetuksi** kertoo, että muutos oli sinun tekemäsi. Uusi taso muuttuu normaaliksi 10 varmuuskopion jälkeen, mutta lähes tyhjän lähteen tarkistusta ei koskaan kytketä pois.
- Muuttunut valinta, esimerkiksi kansio, jonka valitsit tai poistit valinnasta, aloittaa merkinnän historian alusta itsestään.

Ilmoitus lähtee kerran jaksoa kohti, eikä epäonnistumisista ja palautustarkistuksista, jotka jo ilmoittavat, kerrota kahdesti.

## Vanhat varmuuskopiot säilyvät sillä välin {#retention-hold}

Kun lähde on lähes tyhjä, kutistunut jyrkästi tai suurin osa sen datasta on tallennettu uudelleen, säilytys ja karsinta jättävät kyseisen merkinnän vanhat varmuuskopiot rauhaan. Havainto linkittää viimeiseen hyvään varmuuskopioon, jonka palautusikkuna valitsee valmiiksi. Ennen kuin suljet tällaisen havainnon, BombVault kertoo, että seuraava siivous poistaa vanhoja varmuuskopioita taas säilytyssääntöjen mukaan. Jos dataa puuttuu tai se salattiin, palauta siis ensin ja sulje havainto vasta sen jälkeen.

## Asetukset {#settings}

**Poikkeamat**-kortti kohdassa **Asetukset, Tarkista** sisältää yleiset asetukset. Jokainen säädin tallentuu heti, kun muutat sitä, ja kytkimen alla olevat kolme piilotetaan, kun tunnistus on pois päältä.

| Asetus | Oletus | Mitä se tekee |
|---|---|---|
| **Tunnista poikkeamat** | Päällä | Vertaa jokaista varmuuskopiota merkinnän omaan historiaan. Kun asetus on pois päältä, mitään uutta ei tarkisteta ja **Poikkeamat**-kohta poistuu sivupalkista. Kortti linkittää edelleen aiempiin havaintoihin. |
| **Herkkyys** | Tasapainoinen | Tiukka ilmoittaa pienemmistä muutoksista, Salliva vain suurista. |
| **Lähetä ilmoitus, kun kyseessä on** | Vain kriittiset havainnot | Alin vakavuus, joka lähettää viestin kohdassa [Ilmoitukset](notifications.md) määritettyjen kanavien kautta. Toistuvat varmuuskopioinnin ja vedosten epäonnistumiset sekä epäonnistuneet ajastetut palautustarkistukset lähettävät jo oman viestinsä, joten niitä ei lähetetä kahdesti. |
| **Säilytä vanhat varmuuskopiot, kun lähde kutistuu jyrkästi tai kirjoitetaan uudelleen** | Päällä | Kun merkinnällä on avoin havainto lähes tyhjästä lähteestä, jyrkästä kutistumisesta tai siitä, että suurin osa datasta tallennettiin uudelleen, säilytys ja karsinta jättävät kyseisen merkinnän vanhat varmuuskopiot rauhaan. Kuittaa havainto tai merkitse se odotetuksi, jotta ne voidaan päästää menemään. |

Jokaisella merkinnällä voi olla oma herkkyytensä ja ilmoitusten alarajansa. Aseta ne merkinnän **Tarkista**-välilehdellä tai **Poikkeamat**-sivulla kohdassa **Valvonta**. ZFS-merkinnällä ne koskevat kaikkia sen puun tietojoukkoja.
