# Pierwsze kroki

Ta strona przeprowadzi Cię od świeżej maszyny Unraid do Twojej pierwszej kopii zapasowej.

## Wymagania

| Wymaganie | Uwagi |
|---|---|
| **Unraid 6.12+** | Wcześniejsze wersje nie są testowane. |
| **Lokalizacja repozytorium restic** | Ścieżka lokalna (zalecane: Twoja macierz lub cache), SMB, NFS lub dowolny backend rclone. |
| **Gniazdo Docker** | Montowane automatycznie przez szablon (`/var/run/docker.sock`). |
| **Flash Unraid** (`/boot`) | Montowany w całości automatycznie przez szablon (`/boot` do `/host/boot`). Zasila kopię flash i pozwala przywróconemu kontenerowi pojawić się ponownie jako normalna, edytowalna aplikacja Unraid. |
| **Maszyny wirtualne KVM** (opcjonalnie) | Kopia VM komunikuje się z libvirt przez SSH, bez montowania libvirt. Skonfiguruj to w Ustawieniach (zobacz [Konfiguracja](configuration.md)). |

## Instalacja na Unraid

Najprostsza droga to **Community Applications**.

1. Otwórz zakładkę **Apps** w Unraid.
2. Wyszukaj **BombVault**.
3. Kliknij **Install**, ustaw wymagane zmienne (poniżej) i zastosuj.

!!! tip "Ręczna instalacja szablonu"
    Jeśli wolisz dodać szablon ręcznie:

    1. Przejdź do **Docker, Add Container, Template repositories** i dodaj:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Wyszukaj **BombVault** w Templates.
    3. Ustaw wymagane zmienne i kliknij **Apply**.

## Jedno wymagane ustawienie

Jedyną zmienną, którą musisz ustawić, jest `APP_KEY`, 32-bajtowy sekret w formacie hex (64 znaki hex) używany do wyprowadzenia hasła repozytorium restic.

Wygeneruj go na dowolnej maszynie:

```bash
openssl rand -hex 32
```

Wklej wynik do pola `APP_KEY` w szablonie.

!!! danger "Nie zgub swojego APP_KEY"
    Utrata `APP_KEY` sprawia, że zaszyfrowane kopie zapasowe stają się nieodzyskiwalne. Przechowuj go w bezpiecznym miejscu, oddzielnie od serwera. Gdy BombVault już działa, użyj jego funkcji **zestaw odzyskiwania klucza szyfrowania** dostępnej za jednym kliknięciem (zobacz [Kopie poza siedzibą i odzyskiwanie](offsite-recovery.md)), aby zapisać pełny pakiet odzyskiwania.

Szablon montuje też za Ciebie gniazdo Docker, flash (`/boot`) oraz katalog główny **Host Data** (`/mnt`). Zarówno *źródła*, jak i *cele* kopii zapasowych znajdują się pod Host Data. Pełny opis zmiennych oraz konfigurację poza siedzibą znajdziesz w [Konfiguracja](configuration.md).

## Pierwsze uruchomienie

1. Otwórz interfejs webowy pod adresem `https://<your-unraid-ip>:3443` (certyfikat samopodpisany od razu po instalacji).
2. W **Ustawieniach** włącz domeny kopii zapasowych, których chcesz używać (Kontenery, VM, Flash, Config, Pliki) i wybierz kolor akcentu.
3. W zakładce **Kontenery** wybierz kontener i kliknij **Utwórz kopię**, aby stworzyć swój pierwszy punkt przywracania. Ścieżki repozytoriów domyślnie wynoszą `/mnt/user/bombvault/{container,vms,flash,config,files}` i są tworzone przy pierwszej kopii.
4. Skonfiguruj harmonogramowanie w **Ustawienia, Harmonogramy**. Dostępna jest funkcja *uwzględnij wszystkie w harmonogramie* za jednym kliknięciem dla kontenerów i VM.

!!! tip "Opcjonalnie: wybierz kolejność kopii zapasowych"
    Jeśli niektóre kontenery powinny być zawsze kopiowane przed innymi (na przykład baza danych przed aplikacją, która z niej korzysta), otwórz panel **kolejności kopii** na stronie Kontenery i przeciągnij je w wybraną sekwencję. Uruchomienia zaplanowane i wielokrotnego wyboru będą jej przestrzegać; wszystko, co pozostawisz bez kolejności, jest kopiowane od najbardziej zaległych, jak poprzednio.

!!! note "Kontrola integracji z hostem"
    Otwórz `/spike` w interfejsie webowym po uruchomieniu kontenera. Sonduje ono każdy montaż i każde CLI (gniazdo Docker, libvirt, restic, qemu-img, rclone) i zgłasza wszelkie brakujące elementy, więc możesz potwierdzić, że kontener jest poprawnie połączony, zanim na nim polegasz.

## Prosty vs Zaawansowany

Domyślnie interfejs pokazuje tylko rzeczy podstawowe (tworzenie kopii, przywracanie, harmonogram). Użyj przełącznika **Prosty / Zaawansowany** w panelu bocznym, aby odsłonić kontrolki dla ekspertów: przechowywanie, kopię poza siedzibą, haki pre/post, przywracanie na poziomie plików, powiadomienia, metryki Prometheus oraz narzędzia integralności/konserwacji. To preferencja per przeglądarka, domyślnie wyłączona, więc nowicjusze dostają czysty interfejs, a użytkownicy zaawansowani mają wszystko.

## Kolejne kroki

- Przejrzyj pełne **[Funkcje](features.md)**.
- Dodaj jedną lub więcej replik **[Kopie poza siedzibą i odzyskiwanie](offsite-recovery.md)** (każda domena może wysyłać do kilku celów naraz) i zapisz swój zestaw odzyskiwania.
- Klonujesz konfigurację lub przenosisz się na nową maszynę? Przenieś całą swoją konfigurację za pomocą karty **Eksport i import ustawień**. Zobacz [Konfiguracja](configuration.md#portable-settings-export-and-import).
- Napotkałeś problem? Zobacz **[Rozwiązywanie problemów](troubleshooting.md)**.
