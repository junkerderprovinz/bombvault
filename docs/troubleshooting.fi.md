# Vianmääritys

Lyhyt UKK. Täydellistä VM-over-SSH-isäntäpuolen vianmääritystaulukkoa varten (permission-denied, isäntäavaimen todennus, puuttuvat mallimuuttujat ja muuta) katso [VM-varmuuskopiointi SSH:n yli -opas](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) GitHubissa.

## Jokin ei ole kytketty oikein

Avaa `/spike` verkkokäyttöliittymässä. Isäntäintegraation tarkistus koettaa jokaista liitosta ja komentorivityökalua (Docker-soketti, libvirt, restic, qemu-img, rclone) ja raportoi puuttuvat palaset. Aloita täältä ennen kuin oletat bugia: puuttuva liitos tai tavoittamaton isäntä näkyy heti.

## En saa yhteyttä verkkokäyttöliittymään

BombVault tarjoaa HTTPS:ää valmiiksi portissa `3443` (itse allekirjoitettu varmenne), joten avaa `https://<your-unraid-ip>:3443`. Hyväksy itse allekirjoitetun varmenteen varoitus tai sijoita BombVault käänteisen välityspalvelimen taakse omalla varmenteellasi. Jos ajat asetuksella `HTTP_ONLY=true`, se tarjoaa sen sijaan selkeää HTTP:tä portissa `3000` (tarkoitettu käytettäväksi TLS:n päättävän välityspalvelimen takana).

## Menetin APP_KEY:ni

`APP_KEY` johtaa restic-arkiston salasanan. Ilman sitä (ja ilman salausavaimen palautuspakettia) salattuja varmuuskopioita ei voi palauttaa. Tämän vuoksi Kojelauta nalkuttaa sinua lataamaan palautuspaketin. Katso [Etäsijainti ja palautus](offsite-recovery.md). Luo avain komennolla `openssl rand -hex 32` ja säilytä se palvelimen ulkopuolella ennen kuin luotat mihinkään varmuuskopioon.

## VM-varmuuskopiointi ei muodosta yhteyttä

VM-varmuuskopiointi keskustelee libvirtin kanssa SSH:n yli, ei koskaan liitoksen kautta.

- Vahvista, että SSH on käytössä isännällä ja BombVaultin julkinen avain on valtuutettu tiedostossa `/root/.ssh/authorized_keys` (Asetukset, Järjestelmä, VM Backup over SSH näyttää avaimen ja **Test connection** -painikkeen).
- Mukautetussa `br0.x`-verkossa aseta `LIBVIRT_HOST` Unraidin LAN-IP-osoitteeseesi (kontti ei voi tavoittaa isäntää `host.docker.internal`-nimellä siellä). Ota käyttöön **Settings, Docker, Host access to custom networks**.
- Jos vaihdoit Unraidin SSH-porttia, aseta `LIBVIRT_SSH_PORT` vastaamaan.
- Täydellinen vaihe vaiheelta -diagnoosi (tavoitettavuustesti, VLAN-reititys, `Permission denied (publickey)`, `Host key verification failed`) on [VM-varmuuskopiointi SSH:n yli -oppaassa](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Live-VM-tilannevedos ei ajautunut

Live-tilannevedokset tarvitsevat qemu guest agentin asennettuna VM:ään ja levyn sijainniksi `/mnt/cache` (tai `/mnt/diskX`), ei `/mnt/user`. Sammutetussa VM:ssä live palautuu automaattisesti hallittuun. Hallittu varmuuskopio sammuttaa VM:n, varmuuskopioi levyt ja käynnistää sen sitten uudelleen, joten se on aina eheä.

## Varmuuskopio epäonnistui virheeseen "repository is already locked"

Tämä on yleensä orpo restic-lukko, joka jäi jälkeen kun kontti päivitettiin tai käynnistettiin uudelleen kesken toiminnan. BombVault havaitsee todistettavasti orvon lukon, pakottaa sen auki ja yrittää uudelleen kerran, automaattisesti. Jos se jää pysyväksi, käytä **Asetukset, Eheys ja ylläpito, Avaa lukitus** kyseiselle toimialueelle poistaaksesi jumittuneen lukon käsin. Todellinen ongelma nousee silti pintaan sen sijaan että piiloutuisi.

## Etäkopiotani ei tapahtunut varmuuskopion jälkeen

Etäreplikointi on suunnitellusti parhaan yrityksen mukaista, joten etäsijainnin nikottelu ei koskaan kaada paikallista varmuuskopiota. Tarkista kyseisen toimialueen etäaikataulu (Asetukset, Aikataulut): tyhjä aikataulu replikoi jokaisen paikallisen varmuuskopion jälkeen, kun taas tahti lähettää harvemmin. Käytä **Replikoi nyt** Etä-välilehdellä pyydettäessä tehtävään ajoon, ja tarkkaile replikointiosoitinta Kojelaudalla.

## Palautus keskeytyi ennen kuin se alkoi

Ennen kuin mitään pysäytetään tai poistetaan, palautus ajaa esitarkastuksen ristiriidoista: se varmistaa, että kontin kiinteä IP ja julkaistut isäntäportit ovat vapaana. Jos toinen kontti jo pitää yhtä hallussaan, se keskeyttää selkeällä, toimintakelpoisella viestillä sen sijaan että jättäisi puolivalmiin palautuksen. Vapauta ristiriitainen portti tai IP ja yritä sitten uudelleen.

## Selkokielinen vienti epäonnistui tiedoston kirjoittamisen sijaan

Jos age-salaus on päällä (Asetukset) mutta kelvollista vastaanottajaa ei ole asetettu, vienti epäonnistuu selkeällä virheellä selkotekstin kirjoittamisen sijaan. Lisää kelvollinen vastaanottaja (age-julkinen avain tai SSH-julkinen avain), tai kytke salaus pois päältä, jos tarkoitat viennin olevan selkotekstiä. Katso [Ominaisuudet](features.md).

## Kontti käynnistyy jatkuvasti uudelleen tai näyttää epäterveeltä

BombVault raportoi terve/epäterve omasta `/api/health`-päätepisteestään. Automaattinen korjaustyökalu (kuten Autoheal) voi käynnistää sen uudelleen automaattisesti, jos moottori koskaan jumiutuu. Tarkista kontin loki ja `/spike`-raportti taustalla olevan syyn selvittämiseksi.

## Yhä jumissa?

- Lue täydelliset [Asetukset](configuration.md)- ja [Etäsijainti ja palautus](offsite-recovery.md) -sivut.
- Kysy [Unraid-tukiketjussa](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Avaa [GitHub-ongelma](https://github.com/junkerderprovinz/bombvault/issues).
