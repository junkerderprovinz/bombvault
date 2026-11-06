# Beállítások

A **Beállítások** tizennégy csempéből állnak. Minden csempe egy kártyákból álló oldal, és minden változás azonnal kiíródik, ahogy megteszed, a vezérlő rövid felvillanásával, vagy megrázkódásával, ha a szerver elutasítja. Néhány ablak megtartja a **Mentés** gombot, mert félkészen elmenteni veszélyes lenne, például egy bejelentkezést vagy a belépési jelszót. A magyarázatok az egyes vezérlők melletti (i) buborékokban vannak.

A beállításokban bárhol a `/` vagy a Ctrl+K lenyomásával mindben kereshetsz, a korábbi verziókban használt oldalnevekre is. A csempék sorrendje a **Megjelenés** alatt módosítható. Ahol az oldal melletti csempeoszlop összeszorítaná az oldalt, például egy kb. 1200 pixel széles ablakban vagy telefonon, a csempék az oldal fölötti sávvá válnak, amely oldalra görgethető.

![A Beállítások az Általános csempével megnyitva.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![A Beállítások az Általános csempével megnyitva.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Ez az oldal felsorolja az összes csempét és a kártyáikat, és arra az oldalra mutat, amely mindegyiket részletesen elmagyarázza.

## Általános {#general}

- A **Szakaszok** szakaszonként be- vagy kikapcsolja: **Konténerek**, **VM-ek**, **Flash**, **Mappák**, **ZFS-adatkészletek** és **Önmentés**. A kikapcsolt szakaszról semmilyen ütemezés szerint nem készül mentés, egy elem saját tervét is beleértve, és az elemei nem számítanak. A **Biztonsági mentés most** továbbra is működik. A **Flash** csak Unraidon látszik.
- A **Konténerek** azt tartalmazza, ami minden konténerre vonatkozik, például az **Automatikus adatbázis-mentések**. Ami egyetlen konténert érint, az a konténer oldalán van. Lásd: [Adatbázis-mentések](backups.md#database-dumps).

## Megjelenés {#look}

- **Megjelenés**: **Nyelv** (42 nyelv), **Téma** (**Sötét**, **Világos** vagy **Rendszer**), **Sarkok**, **Animáció** (**Ki**, **Visszafogott**, **Vad** vagy **Viharos**), **Csendes értesítések**, amely elrejti a sikerértesítéseket, például a mentés visszajelzéseit, és **A beállítási oldalak sorrendje**, ahol egy csempét megfogsz és a helyére húzod, vagy az Alt és a nyílbillentyűk segítségével mozgatod.
- **Feliratok**: mennyit mutat egy vezérlőből, külön a **Gombok**, az **Oldalsáv**, a **Lapok** és az **Alsó sáv** esetén: **Szöveg**, **Szöveg és szimbólum**, **Szimbólum**, vagy **Reaktív**, amely csak a szimbólumot mutatja, és becsúsztatja a szavakat, ha rámutatsz. Az **Oldalsáv** csak az asztali elrendezést érinti, az **Alsó sáv** csak a telefonosat.
- **Színek**: egy **Kiemelőszín**, vagy a **Szivárvány mód**, ahol a lista minden sora saját színt kap egy nyolcszínű palettáról. A Szivárvány mód hozzáadja a **Diszkó módot**, ahol a színek lassan átsiklanak a következőre, a **Reaktív módot**, ahol a sor színe csak akkor látszik, ha rámutatsz, fut, vagy ki van jelölve, a **Színforgatást** és magát a **Színpalettát**.

A BombVault a megjelenést a szerveren tárolja. Az első alkalommal érkező böngésző átveszi. Ha a rendszered kevesebb mozgást kér, az mindig erősebb az **Animációnál**.

## Ütemezések {#schedule}

**Hét**, **Teljes biztonsági mentés** a **Mikor futnak a mentések**, az **Elmaradt biztonsági mentések pótlása indulás után** és a **Parancsok előtte és utána** beállítással, **Ütemezések szakaszonként** a **Telephelyen kívüli másolatok szakaszonként** résszel, **Ütemezett elemek**, **Sorrend** az **Újraindítás biztonsági mentés után** résszel, **Pillanatkép leállítás helyett**, **Frissítések a mentés után** és **Nyugalom biztonsági mentés előtt**. Lásd: [Ütemezések](schedules.md).

## Célok {#storage-locations}

- A **Mutatás az oldalsávban is** kitűzi a Célok oldalt az oldalsávra.
- A céljaid listája a **Cél hozzáadása** gombbal. Egy kattintás megnyitja a cél oldalát, lásd: [Egy cél oldala](offsite-recovery.md#location-page).
- **Bejelentkezések**: az elmentett bejelentkezések az alapértelmezett csillaggal, lásd: [Bejelentkezések](offsite-recovery.md#sign-ins).
- **Célok szakaszonként**: a 3-2-1 folyamat, ahol minden sor egy kapcsoló, lásd: [Alapértelmezések szakaszonként](offsite-recovery.md#placement-defaults).
- **Tárolók**: az egyes elemekhez készült tárolók, lásd: [Elhelyezés elemenként](offsite-recovery.md#placement).
- A **Meddig őrizze meg** a célokra mutat, ahol a megőrzés be van állítva, az **Előnézet** pedig megmutatja a **Mit törölne a következő futás** listát, tárolónként és elemenként, mielőtt megtörténne. Semmit nem változtat és semmit nem zár le, ezért futó mentés közben is válaszol.
- **Telephelyen kívüli másolatok**: szakaszonként egy sor a **Kapcsolat tesztelése**, a **Replikálás most** és a **Beállítás** vagy **Szerkesztés** gombbal, lásd: [Telephelyen kívüli másolatok](offsite-recovery.md#off-site-copies).
- **Telephelyen kívüli sávszélesség**: **Feltöltési korlát (KiB/s)** és **Letöltési korlát (KiB/s)** minden másolatra. A 0 korlátlant jelent.

A **Streamelés** kártya, amely lejjebb viszi a feltöltési korlátot, amíg a Plex, a Jellyfin vagy az Emby streamel, magán a Célok oldalon áll.

## Visszaállítás {#restore}

- Egy hivatkozás a **Visszaállítás** oldalra.
- **Visszaállítási mappák**: az **Alapértelmezett visszaállítási mappa** egy mappába történő visszaállításhoz, az **Almappa szakaszonként** beállítással és szakaszonkénti **Kivétellel**. Lásd: [Hová kerül egy mappába történő visszaállítás](restore.md#folders).
- **Container registry-k**: bejelentkezések privát registry-khez, például egy szponzorokhoz kötött `ghcr.io` image-hez. A visszaállítás ezekkel tölti le a hiányzó image-et, a mentés utáni frissítés pedig ezekkel ellenőrzi, van-e újabb. A nyilvános image-ek bejelentkezés nélkül töltődnek le. A **Registry hozzáadása** nyilvános registry-ket, saját üzemeltetésűeket, felhőszolgáltatóknál lévőket és a **Másik registry** lehetőséget kínálja bármely címhez.
- **Vészhelyzetre**: **Helyreállítási csomag letöltése**, amihez belépési jelszó kell. Lásd: [Helyreállítási csomag](offsite-recovery.md#recovery-kit).

## Értesítések {#notifications}

**Értesítések** az **Értesítés**, az összegzések, a **Késő mentések jelentése**, az **Unraid-értesítések** és a **Heti összefoglaló** beállításokkal, **Csatornák** a Webhook, Apprise, Matrix, Gotify és E-mail (SMTP) csatornákkal, és **Healthchecks**. Lásd: [Értesítések](notifications.md).

## Ellenőrzés {#verify}

- **Mentések ellenőrzése**: az utolsó `restic check` eredménye szakaszonként, a **Feloldás** és a **Nyesés** műveletekkel a **Több** alatt minden sorban. A **Feloldás** feloldja a megmaradt zárat, a **Nyesés** pedig kérésre alkalmazza a megőrzési szabályokat.
- **Automatikus ellenőrzések**: az ütemezett visszaállítás-ellenőrzés az **Adatminta (%)** értékével és a teszt célokkal, az **Indítási teszt**, amely egy konténert is visszaállít egy elszigetelt másolatba, és elindítja, az **Ütemezett telephelyen kívüli DR-próba** és a **Törlésvédelem ellenőrzése**, az ütemezett manipulációs teszt. Lásd: [DR-próbák](offsite-recovery.md#dr-drills) és [Manipulációs teszt](offsite-recovery.md#tamper-test).
- **Anomáliák**: a felismerés, az érzékenység, és hogy mely leletek küldenek üzenetet. Lásd: [Anomáliák](anomalies.md).

## Biztonság {#security}

- **Jelszó**: a belépési jelszó, legalább 12 karakter. Megváltoztatása vagy törlése a jelenlegit kéri, és bekapcsolt kétlépcsős azonosításnál a kódot is.
- **Kétlépcsős azonosítás**: egy hitelesítő alkalmazás kódja a jelszó mellett, nyolc helyreállítási kóddal, amelyeket egyszer mutat meg.
- **Hozzáférési kulcsok**: bejelentkezés a telefonodon, laptopodon vagy biztonsági kulcsodon lévő kulccsal. Valódi domainnévre és a böngésző által elfogadott tanúsítványra van szükségük.
- **Android-alkalmazás**: **A telefonok párosíthatók a csoport tizenkét szavával**, ami lehetővé teszi, hogy egy párosított telefon jelszó nélkül jelentkezzen be.
- **Titkosítás**: **Mentések titkosítása**, alapértelmezés szerint bekapcsolva.

Néhány dologhoz belépési jelszó kell, például a helyreállítási csomaghoz és a kulcsletétekhez. Lásd: [Biztonsági modell](configuration.md#security-model).

## Példányok {#instances}

- A **Mutatás az oldalsávban is** kitűzi a Példányok oldalt az oldalsávra.
- A **Több szerver** bekapcsolja egy csoport részeit: a **Példányok** oldalt, amelyen minden taghoz tartozik egy kártya, a **Fogadót** és a **Lehívást**. Lásd: [Példányok és szerepek](offsite-recovery.md#roles).
- Egy hivatkozás a Példányok oldalra, a bekapcsolt részekkel.

## Párosítás {#pairing}

A **Párosítás** a **Jelmondat létrehozása** vagy a **Jelmondat megadása** lehetőséggel, a lépéskártyák, ennek a példánynak a neve és az **Ismert címek**, a **Relay** kártya a négy formájával és a **Mit lát a relay** résszel, a csoport tagjai, valamint a **ZFS-kiszolgáló BombVault nélkül** egy SSH-n át elért replikacélhoz. Lásd: [Példányok párosítása](offsite-recovery.md#pairing) és [Küldés egy ZFS-kiszolgálóra](zfs-datasets.md#replica-push).

## Integrációk {#integrations}

- **MCP-kiszolgáló**: kulcsok MI-asszisztensekhez, **Kliens kiválasztása** és az egyes kulcsok naplója. Lásd: [MCP-kiszolgáló](mcp.md).
- **API-tokenek**: elnevezett tokenek szkriptekhez és irányítópultokhoz. Lásd: [Tokenek](api.md#tokens).
- **Szolgáltatások**: **Home Assistant** MQTT-n át, **Megtalálás a hálózaton** (mDNS `bombvault.local` néven) és a **/metrics közzététele** a Prometheushoz, **Lekérdezési token (opcionális)** beállítással. Lásd: [Home Assistant](api.md#home-assistant) és [A BombVault megtalálása a hálózaton](api.md#mdns).
- **Irányítópult-widget**: egy csak olvasható token és egy cím bármely iframe-et mutató irányítópulthoz, például a Homepage-hez vagy a Heimdallhoz, csak az élő tevékenységnaplóval. Lásd: [Irányítópult-widget](api.md#dashboard-widget).
- **Gazdagép SSH**: a BombVault nyilvános kulcsa és a **Kapcsolat tesztelése**, a VM- és ZFS-adatkészlet-mentésekhez, valamint az Unraid-értesítésekhez. Lásd: [VM-mentési útmutató](vm-backup-ssh-setup.md).

## Alkalmazások {#apps}

- **Android-alkalmazás**: a szervered kiadásához tartozó APK QR-kóddal, és az F-Droid-lista. Lásd: [Android-alkalmazás](android.md#install).
- **ParleyPort**: a relay konténer az Unraid-sablonjával és az őt elindító Docker-paranccsal.
- **BombVault Widget**: egy csempe az Unraid vezérlőpultján a BombVault tevékenységnaplójával és a következő futással. Gazdagép SSH-kapcsolattal a kártya maga telepíti és távolítja el a bővítményt. Anélkül megadja a `.plg` címet, amelyet a **Plugins, Install Plugin** alatt telepíthetsz.

## Rendszer {#system}

- **Beállítások exportálása / importálása**: a teljes konfigurációd hordozható JSON-fájlként, a hitelesítő adatokkal vagy nélkülük. Az import először az **Ez a fájl tartalmazza** részt mutatja, és kihagyja a már nem létező célokra mutató hivatkozásokat. Az **Exportok titkosítása age-dzsel** lezárja a sima exportokat, a flash ZIP-et és a helyreállítási csomagot az általad felsorolt **age címzettek** számára. Lásd: [Hordozható beállítások](configuration.md#portable-settings-export-and-import).
- **Teljesítmény**: **CPU szálak (0 = mind)**, **Gyorsítótár-méretkorlát (MB, 0 = korlátlan)** a restic `/config` alatti gyorsítótárához, és **Memóriakorlát biztonsági mentésenként (MiB, 0 = nincs)**, egy lágy korlát, amely miatt egy nagy mentés lelassul ahelyett, hogy memóriahiány miatt kilőnék.

## Információ {#info}

- **Verzió**: BombVault, GlimStone, restic és rclone, a **Frissítések keresése** és az **Újdonságok** gombbal, amely megszámolja a még nem látott változásokat, és megnyit egy ablakot, amely felsorolja őket.
- **Súgó**: a **Kézikönyv** megnyitja ezt a dokumentációt, a **Hibajelentés** pedig letölt egy kitakart ZIP-et egy GitHub-hibajegyhez. Belépési jelszó kell hozzá.
- **A BombVault névjegye**: a projekt, a licence és az, hogyan érhető el a szerző.
