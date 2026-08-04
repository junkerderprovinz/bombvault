# Rozwiązywanie problemów

Krótkie FAQ. Pełną tabelę rozwiązywania problemów po stronie hosta dla VM przez SSH (permission-denied, weryfikacja klucza hosta, brakujące zmienne szablonu i więcej) znajdziesz w [przewodniku Kopia VM przez SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) na GitHub.

## Coś nie jest poprawnie połączone

Otwórz `/spike` w interfejsie webowym. Kontrola integracji z hostem sonduje każdy montaż i każde CLI (gniazdo Docker, libvirt, restic, qemu-img, rclone) i zgłasza wszelkie brakujące elementy. Zacznij tutaj, zanim założysz, że to błąd: brakujący montaż lub nieosiągalny host pojawia się natychmiast.

## Nie mogę dotrzeć do interfejsu webowego

BombVault serwuje HTTPS od razu po instalacji na porcie `3443` (certyfikat samopodpisany), więc otwórz `https://<your-unraid-ip>:3443`. Zaakceptuj ostrzeżenie o certyfikacie samopodpisanym lub umieść BombVault za odwrotnym proxy z własnym certyfikatem. Jeśli uruchamiasz z `HTTP_ONLY=true`, serwuje zamiast tego zwykły HTTP na porcie `3000` (przeznaczony do użytku za proxy terminującym TLS).

## Straciłem swój APP_KEY

`APP_KEY` wyprowadza hasło repozytorium restic. Bez niego (i bez zestawu odzyskiwania klucza szyfrowania) zaszyfrowanych kopii nie da się odzyskać. Dlatego panel nęka Cię, abyś pobrał zestaw odzyskiwania. Zobacz [Kopie poza siedzibą i odzyskiwanie](offsite-recovery.md). Wygeneruj klucz poleceniem `openssl rand -hex 32` i przechowuj go poza serwerem, zanim zaczniesz polegać na jakiejkolwiek kopii.

## Kopia VM nie łączy się

Kopia VM komunikuje się z libvirt przez SSH, nigdy przez montaż.

- Potwierdź, że SSH jest włączone na hoście, a klucz publiczny BombVault jest autoryzowany w `/root/.ssh/authorized_keys` (Ustawienia, System, Kopia VM przez SSH pokazuje klucz oraz przycisk **Test połączenia**).
- W niestandardowej sieci `br0.x` ustaw `LIBVIRT_HOST` na swoje IP Unraid w LAN (kontener nie może tam dotrzeć do hosta przez `host.docker.internal`). Włącz **Settings, Docker, Host access to custom networks**.
- Jeśli zmieniłeś port SSH Unraid, ustaw `LIBVIRT_SSH_PORT`, aby pasował.
- Pełna diagnoza krok po kroku (test osiągalności, routing VLAN, `Permission denied (publickey)`, `Host key verification failed`) znajduje się w [przewodniku Kopia VM przez SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Migawka VM na żywo nie została uruchomiona

Migawki na żywo wymagają agenta gościa qemu zainstalowanego w VM oraz dysku na `/mnt/cache` (lub `/mnt/diskX`), nie `/mnt/user`. Na wyłączonej VM tryb na żywo automatycznie wraca do płynnego. Płynna kopia wyłącza VM, tworzy kopię dysków, a następnie ją restartuje, więc jest zawsze spójna.

## Kopia zapasowa zawiodła z "repository is already locked"

To zwykle osierocona blokada restic pozostawiona, gdy kontener został zaktualizowany lub zrestartowany w trakcie operacji. BombVault wykrywa bezspornie osieroconą blokadę, wymuszenie ją usuwa i ponawia raz, automatycznie. Jeśli się utrzymuje, użyj **Ustawienia, Integralność i konserwacja, Odblokuj** dla dotkniętej domeny, aby ręcznie usunąć nieaktualną blokadę. Prawdziwy problem nadal wychodzi na jaw zamiast być ukrywany.

## Moja kopia poza siedzibą nie zdarzyła się po kopii

Replikacja poza siedzibą jest z założenia best-effort, więc potknięcie poza siedzibą nigdy nie powoduje niepowodzenia kopii lokalnej. Sprawdź harmonogram poza siedzibą dla tej domeny (Ustawienia, Harmonogramy): pusty harmonogram replikuje po każdej kopii lokalnej, podczas gdy kadencja wysyła rzadziej. Użyj **Replikuj teraz** w zakładce Poza siedzibą do uruchomienia na żądanie i obserwuj wskaźnik replikacji na panelu.

## Przywracanie przerwane, zanim się zaczęło

Zanim cokolwiek zostanie zatrzymane lub usunięte, przywracanie uruchamia kontrolę konfliktów przed startem: weryfikuje, czy statyczny IP kontenera oraz opublikowane porty hosta są wolne. Jeśli inny kontener już jeden z nich zajmuje, przerywa działanie z czytelnym, praktycznym komunikatem zamiast pozostawiać niedokończone przywracanie. Zwolnij konfliktujący port lub IP, a następnie ponów.

## Jawny eksport zawiódł zamiast zapisać plik

Jeśli szyfrowanie age jest włączone (Ustawienia), ale nie ustawiono prawidłowego odbiorcy, eksport kończy się czytelnym błędem zamiast zapisać tekst jawny. Dodaj prawidłowego odbiorcę (klucz publiczny age lub klucz publiczny SSH) albo wyłącz szyfrowanie, jeśli chcesz, aby eksport był w postaci jawnej. Zobacz [Funkcje](features.md).

## Kontener wciąż się restartuje lub wygląda na niesprawny

BombVault zgłasza stan healthy/unhealthy z własnego `/api/health`. Narzędzie do auto-naprawy (takie jak Autoheal) może go automatycznie zrestartować, jeśli silnik kiedykolwiek się zaklinuje. Sprawdź log kontenera oraz raport `/spike` w poszukiwaniu przyczyny źródłowej.

## Nadal utknąłeś?

- Przeczytaj pełne strony [Konfiguracja](configuration.md) i [Kopie poza siedzibą i odzyskiwanie](offsite-recovery.md).
- Zapytaj na [wątku wsparcia Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Otwórz [zgłoszenie na GitHub](https://github.com/junkerderprovinz/bombvault/issues).
