# Aloitus

Tämä sivu opastaa sinut tuoreesta Unraid-laatikosta ensimmäiseen varmuuskopioosi.

## Vaatimukset

| Vaatimus | Huomiot |
|---|---|
| **Unraid 6.12+** | Aiempia versioita ei ole testattu. |
| **Restic-repon sijainti** | Paikallinen polku (suositus: array tai cache), SMB, NFS tai mikä tahansa rclone-taustajärjestelmä. |
| **Docker-soketti** | Malli liittää automaattisesti (`/var/run/docker.sock`). |
| **Unraid flash** (`/boot`) | Malli liittää kokonaan automaattisesti (`/boot` kohteeseen `/host/boot`). Mahdollistaa flash-varmuuskopioinnin ja sen, että palautettu kontti ilmestyy takaisin normaalina, muokattavana Unraid-sovelluksena. |
| **KVM-virtuaalikoneet** (valinnainen) | VM-varmuuskopiointi keskustelee libvirtin kanssa SSH:n yli, ei libvirt-liitosta. Määritä se Asetuksissa (katso [Asetukset](configuration.md)). |

## Asennus Unraidiin

Helpoin reitti on **Community Applications**.

1. Avaa **Apps**-välilehti Unraidissa.
2. Hae **BombVault**.
3. Napsauta **Install**, aseta vaaditut muuttujat (alla) ja ota käyttöön.

!!! tip "Mallin manuaalinen asennus"
    Jos haluat lisätä mallin käsin:

    1. Mene kohtaan **Docker, Add Container, Template repositories** ja lisää:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Hae **BombVault** kohdasta Templates.
    3. Aseta vaaditut muuttujat ja napsauta **Apply**.

## Yleinen Docker-isäntä

Ei Unraidia? BombVault toimii myös tavallisena konttina millä tahansa Docker-isännällä (tämä kannattelee myös TrueNAS Scalen konttituen, ennen kuin sillä on oma merkintänsä sikäläisessä sovelluskatalogissa).

1. Nouda arkistosta valmiiksi muokattava [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml).
2. Aseta `APP_KEY` (katso alta) ja osoita Host Data -taltio todelliseen datajuureesi: tiedoston kommentit käyvät läpi molemmat.
3. `docker compose up -d`, avaa sitten `https://<isännän-ip>:3443/`.

Mikä poikkeaa Unraidista:

- **Ei flash/USB-aluetta.** Käynnistys-USB:tä ei ole talteen otettavaksi tai palautettavaksi, joten asetusten Flash-alueella ei ole täällä tehtävää. Sen sijaan Tiedostot-alue tarjoaa yhden napsautuksen ehdotuksen **Lisää esiasetus: isäntäjärjestelmän kokoonpano** (aloittava `/etc`-tiedostojoukko, jonka käyt läpi ja muokkaat ennen tallennusta) käytännöllisenä yleisenä vastineena.
- **Ei Unraidin omia ilmoituksia.** BombVaultin omat ilmoituskanavat (webhook, off-site-epäonnistumisen varoitukset ja niin edelleen) toimivat tavalliseen tapaan; pois jää vain Unraid-kohtainen lähetys sen omaan ilmoitusjärjestelmään, koska sellaista järjestelmää ei täällä ole.
- **Virtuaalikoneiden varmuuskopiointi on valinnaista ja vaatii erillisen, SSH:n yli tavoitettavan libvirtd-isännän.** Katso compose-tiedoston kommentoitu lohko. Yleisessä Docker-isännässä ei ole omaa virtuaalikoneiden hallintaa.

## Ainoa vaadittu asetus

Ainoa muuttuja, joka sinun on asetettava, on `APP_KEY`, 32-tavuinen heksadesimaalisalaisuus (64 heksamerkkiä), jota käytetään restic-arkiston salasanan johtamiseen.

Luo sellainen millä tahansa koneella:

```bash
openssl rand -hex 32
```

Liitä tulos mallin `APP_KEY`-kenttään.

!!! danger "Älä menetä APP_KEY:tä"
    `APP_KEY`:n menettäminen tekee salatuista varmuuskopioistasi palautuskelvottomia. Säilytä se turvallisessa paikassa erillään palvelimesta. Kun BombVault on käynnissä, käytä sen yhden napsautuksen **salausavaimen palautuspakettia** (katso [Etäsijainti ja palautus](offsite-recovery.md)) tallentaaksesi koko palautusnipun.

Malli liittää myös Docker-soketin, flashin (`/boot`) ja **Host Data** -juuren (`/mnt`) puolestasi. Varmuuskopioinnin *lähteet* ja *kohteet* asuvat molemmat Host Datan alla. Täydellinen muuttujaviite ja etäsijainnin määritys löytyvät kohdasta [Asetukset](configuration.md).

## Ensimmäinen ajo

1. Avaa verkkokäyttöliittymä osoitteessa `https://<your-unraid-ip>:3443` (itse allekirjoitettu varmenne valmiiksi).
2. Ota **Asetuksissa** käyttöön haluamasi varmuuskopioinnin toimialueet (Kontit, Virtuaalikoneet, Flash, Config, Tiedostot) ja valitse korostusväri.
3. Valitse **Kontit**-välilehdellä kontti ja napsauta **Varmuuskopioi** tehdäksesi ensimmäisen palautuspisteesi. Repopolut ovat oletuksena `/mnt/user/bombvault/{container,vms,flash,config,files}` ja ne luodaan ensimmäisen varmuuskopion yhteydessä.
4. Määritä ajastus kohdasta **Asetukset, Aikataulut**. Konteille ja virtuaalikoneille on yhden napsautuksen *sisällytä kaikki aikatauluun*.

!!! tip "Valinnaista: valitse varmuuskopiojärjestys"
    Jos jotkin kontit tulisi aina varmuuskopioida ennen muita (esimerkiksi tietokanta ennen sitä käyttävää sovellusta), avaa **varmuuskopiojärjestys**-paneeli Kontit-sivulla ja vedä ne haluamaasi järjestykseen. Ajoitetut ja monivalinnalla käynnistetyt ajot noudattavat sitä; kaikki järjestämättä jättämäsi varmuuskopioidaan erääntyneimmät ensin, kuten ennenkin.

!!! note "Isäntäintegraation tarkistus"
    Avaa `/spike` verkkokäyttöliittymässä kontin käynnistyttyä. Se koettaa jokaista liitosta ja komentorivityökalua (Docker-soketti, libvirt, restic, qemu-img, rclone) ja raportoi puuttuvat palaset, joten voit vahvistaa, että kontti on kytketty oikein ennen kuin luotat siihen.

## Yksinkertainen vs Edistynyt

Oletuksena käyttöliittymä näyttää vain olennaiset (varmuuskopiointi, palautus, ajastus). Käytä sivupalkin **Yksinkertainen / Edistynyt** -kytkintä paljastaaksesi asiantuntijasäätimet: säilytys, etäkopio, ennen/jälkeen-koukut, tiedostotason palautus, ilmoitukset, Prometheus-mittarit sekä eheys- ja ylläpitotyökalut. Se on selainkohtainen asetus ja oletuksena pois päältä, joten uudet käyttäjät saavat siistin käyttöliittymän ja tehokäyttäjät kaiken.

## Seuraavat vaiheet

- Selaa täyttä **[Ominaisuudet](features.md)**-listaa.
- Lisää yksi tai useampi **[Etäsijainti ja palautus](offsite-recovery.md)** -replika (kukin toimialue voi lähettää useaan kohteeseen kerralla) ja tallenna palautuspakettisi.
- Kloonaatko kokoonpanoa tai siirrytkö uuteen laatikkoon? Kanna koko kokoonpanosi mukanasi **Vie ja tuo asetukset** -kortilla. Katso [Asetukset](configuration.md#portable-settings-export-and-import).
- Törmäsitkö esteeseen? Katso **[Vianmääritys](troubleshooting.md)**.
