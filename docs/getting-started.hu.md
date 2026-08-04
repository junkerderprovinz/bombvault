# Kezdő lépések

Ez az oldal végigvezet egy friss Unraid-géptől az első mentésedig.

## Követelmények

| Követelmény | Megjegyzések |
|---|---|
| **Unraid 6.12+** | A korábbi verziók nincsenek tesztelve. |
| **Restic tároló helye** | Helyi útvonal (ajánlott: a tömböd vagy a gyorsítótárad), SMB, NFS vagy bármely rclone backend. |
| **Docker socket** | A sablon automatikusan csatolja (`/var/run/docker.sock`). |
| **Unraid flash** (`/boot`) | A sablon automatikusan, teljes egészében csatolja (`/boot` a `/host/boot` alá). Ez teszi lehetővé a flash-mentést, és azt, hogy egy visszaállított konténer normál, szerkeszthető Unraid-alkalmazásként jelenjen meg újra. |
| **KVM VM-ek** (opcionális) | A VM-mentés SSH-n keresztül kommunikál a libvirttel, nincs libvirt-csatolás. Állítsd be a Beállításokban (lásd: [Konfiguráció](configuration.md)). |

## Telepítés Unraidre

A legegyszerűbb út a **Community Applications**.

1. Nyisd meg az **Apps** fület az Unraidben.
2. Keress rá a **BombVault** kifejezésre.
3. Kattints az **Install** gombra, állítsd be a szükséges változókat (lásd lent), majd alkalmazd.

!!! tip "Kézi sablontelepítés"
    Ha inkább kézzel adnád hozzá a sablont:

    1. Menj a **Docker, Add Container, Template repositories** menübe, és add hozzá:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Keress rá a **BombVault** kifejezésre a Templates alatt.
    3. Állítsd be a szükséges változókat, és kattints az **Apply** gombra.

## Az egyetlen kötelező beállítás

Az egyetlen változó, amelyet be kell állítanod, az `APP_KEY`, egy 32 bájtos hexadecimális titok (64 hexadecimális karakter), amely a restic tároló jelszavának származtatására szolgál.

Generálj egyet bármely gépen:

```bash
openssl rand -hex 32
```

Illeszd be az eredményt a sablon `APP_KEY` mezőjébe.

!!! danger "Ne veszítsd el az APP_KEY-t"
    Az `APP_KEY` elvesztése visszaállíthatatlanná teszi a titkosított mentéseidet. Tárold biztonságos helyen, a szervertől elkülönítve. Amint a BombVault fut, használd az egykattintásos **titkosításikulcs-helyreállító csomagját** (lásd: [Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md)) a teljes helyreállítási csomag elmentéséhez.

A sablon ezen felül csatolja a Docker socketet, a flasht (`/boot`) és a **Host Data** gyökeret (`/mnt`) is helyetted. A mentési *források* és *célok* egyaránt a Host Data alatt találhatók. A teljes változó-referenciáért és a telephelyen kívüli beállításért lásd: [Konfiguráció](configuration.md).

## Első futtatás

1. Nyisd meg a webes felületet a `https://<your-unraid-ip>:3443` címen (alapból önaláírt tanúsítvánnyal).
2. A **Beállításokban** engedélyezd a kívánt mentési tartományokat (Konténerek, VM-ek, Flash, Config, Fájlok), és válassz egy kiemelőszínt.
3. A **Konténerek** fülön válassz egy konténert, és kattints a **Mentés** gombra az első visszaállítási pont létrehozásához. A tároló útvonalai alapértelmezetten a `/mnt/user/bombvault/{container,vms,flash,config,files}` útvonalra mutatnak, és az első mentéskor jönnek létre.
4. Állítsd be az ütemezést a **Beállítások, Ütemezések** alatt. A konténerekhez és VM-ekhez van egykattintásos *összes felvétele az ütemezésbe* lehetőség.

!!! tip "Opcionális: válassz mentési sorrendet"
    Ha egyes konténereket mindig más konténerek előtt kell menteni (például egy adatbázist az azt használó alkalmazás előtt), nyisd meg a **mentési sorrend** panelt a Konténerek oldalon, és húzd őket a kívánt sorrendbe. Az ütemezett és a többszörös kijelöléses futások ezt követik; amit rendezetlenül hagysz, azt a korábbi módon a legrégebben esedékes elve szerint menti.

!!! note "Hosztintegráció-ellenőrzés"
    A konténer elindulása után nyisd meg a `/spike` oldalt a webes felületen. Ez minden csatolást és CLI-t megvizsgál (Docker socket, libvirt, restic, qemu-img, rclone), és jelenti a hiányzó darabokat, így megbizonyosodhatsz róla, hogy a konténer helyesen van bekötve, mielőtt rá hagyatkoznál.

## Egyszerű vs Speciális

Alapértelmezetten a felület csak a lényeget mutatja (mentés, visszaállítás, ütemezés). Használd az **Egyszerű / Speciális** kapcsolót az oldalsávban a szakértői vezérlők felfedéséhez: megőrzés, telephelyen kívüli másolat, mentés előtti/utáni horgok, fájlszintű visszaállítás, értesítések, Prometheus-metrikák és az integritási/karbantartási eszközök. Ez böngészőnkénti beállítás, és alapból ki van kapcsolva, így az újoncok tiszta felületet, a haladók pedig mindent megkapnak.

## Következő lépések

- Böngészd a teljes **[Funkciók](features.md)** oldalt.
- Adj hozzá egy vagy több **[Telephelyen kívüli mentés és helyreállítás](offsite-recovery.md)** replikát (minden tartomány egyszerre több célra is szállíthat), és mentsd el a helyreállítási csomagodat.
- Egy beállítást klónozol, vagy új gépre költözöl? Vidd át a teljes konfigurációdat az **Exportálás és importálás beállítások** kártyával. Lásd: [Konfiguráció](configuration.md#portable-settings-export-and-import).
- Elakadtál? Lásd: **[Hibaelhárítás](troubleshooting.md)**.
