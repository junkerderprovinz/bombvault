# Avvikelser

Ransomware och en trasig app ser likadana ut i en säkerhetskopia: plötsligt är det mesta av datan nytt, eller så är det mesta borta. BombVault jämför varje säkerhetskopia med den postens egen historik och skapar ett fynd när något ser fel ut. Så länge ett fynd om förlorad data är öppet behåller den också postens gamla säkerhetskopior, så att den sista goda inte hinner gallras bort. Posten **Avvikelser** i sidofältet visas medan upptäckten är påslagen.

![Sidan Avvikelser med ett kritiskt fynd öppet.](assets/screenshots/anomalies/page-dark.png#only-dark)
![Sidan Avvikelser med ett kritiskt fynd öppet.](assets/screenshots/anomalies/page-light.png#only-light)

## Sidan {#page}

Rutorna **Kritisk**, **Varningar** och **Noteringar** väljer en allvarlighetsgrad, och **Öppna** och **Stängda** ovanför listan växlar mellan det som är öppet och det som stängdes de senaste 30 dagarna. Ett kritiskt fynd ligger på samma yta som de andra, och dess allvarlighetsgrad syns i dess märke och dess färgade kant. Ett fynd erbjuder sin åtgärd först, och dess detaljer visar värdet som en kurva över det vanliga intervallet.

När inget är öppet visar **Inlärningsförlopp** en ring med tio delar per post, en del för varje säkerhetskopia som BombVault lärt sig av. **Övervakning** öppnar listan över poster med sina egna inställningar.

## Vad den kontrollerar {#checks}

Varje säkerhetskopia av en container, VM, mappuppsättning, databasdump, flashenheten och auto-säkerhetskopian jämförs med postens egen historik. Kontrollerna tittar på:

- den nya data en körning lagrade, mot de största vanliga mängderna i de senaste säkerhetskopiorna och mot den vanliga takten per timme,
- en säkerhetskopia som lagrade det mesta av datan på nytt, vilket inkluderar filer som bytt namn och skrivits om,
- källans storlek och antal filer som restic rapporterar för varje post och varje databasdump,
- restics egen säkerhetskopieringstid,
- felserier och återkommande fel,
- återställningskontroller som slutat lyckas,
- det lediga utrymmet i lokala repositorier, SFTP-repositorier och rclone-repositorier, beräknat utifrån repositoriets tillväxt.

En post lär sig av sina första 10 säkerhetskopior. En nästan tom källa, en omskrivning av det mesta av datan och fel kontrolleras från start.

Ett fynd om en källas storlek eller antal filer dateras till den första säkerhetskopian där det syntes. **Jämför med kopian före** listar de mappar där filer försvann, tillkom eller ändrades, med en anmärkning när nästan allt ligger i ett sökindex, en cache eller miniatyrer som appen bygger upp igen själv.

ZFS-poster bevakas datauppsättning för datauppsättning, se [ZFS-datauppsättningar](zfs-datasets.md#anomalies). En assistent kan läsa de öppna fynden via [MCP-servern](mcp.md#tools).

Kontrollerna har gränser. S3-, B2- och REST-repositorier har inget värde för ledigt utrymme, på Unraids användarresurs är det lediga utrymmet hela arrayens, och säkerhetskopior gjorda före restic 0.17 saknar storlekshistorik.

## Stänga ett fynd {#closing}

- **Varningar** stängs av sig själva när orsaken är borta.
- **Kritiska** fynd om förlorad data och en fyllnande disk ligger kvar tills du agerar.
- **Kvittera** markerar ett fynd som sett och stänger det. Det tas inte upp igen förrän orsaken har försvunnit en gång.
- **Markera som väntad** säger att ändringen var din. Den nya nivån blir normal efter 10 säkerhetskopior, men kontrollen för nästan tomt stängs aldrig av.
- Ett ändrat urval, till exempel en mapp du bockat i eller ur, startar postens historik på nytt av sig själv.

En avisering skickas en gång per förlopp, och fel och återställningskontroller som redan skickar avisering rapporteras inte två gånger.

## Gamla säkerhetskopior behålls under tiden {#retention-hold}

Medan en källa är nästan tom, har krympt kraftigt eller har fått det mesta av sin data lagrad på nytt lämnar gallring och prune postens gamla säkerhetskopior ifred, och fyndet länkar till den sista goda säkerhetskopian, som återställningsfönstret sedan förväljer. Innan du stänger ett sådant fynd säger BombVault att nästa städning åter raderar gamla säkerhetskopior enligt behållreglerna. Så om data saknas eller har krypterats, återställ först och stäng fyndet efteråt.

## Inställningar {#settings}

Kortet **Avvikelser** under **Inställningar, Verifiera** innehåller de globala inställningarna. Varje reglage sparas så fort du ändrar det, och de tre under omkopplaren är dolda medan upptäckten är av.

| Inställning | Standard | Vad den gör |
|---|---|---|
| **Upptäck avvikelser** | På | Jämför varje säkerhetskopia med postens egen historik. Avstängd kontrolleras inget nytt och posten **Avvikelser** lämnar sidofältet. Kortet länkar fortfarande till tidigare fynd. |
| **Känslighet** | Balanserad | Strikt rapporterar mindre ändringar, Tillåtande bara stora. |
| **Skicka avisering för** | Bara kritiska fynd | Den lägsta allvarlighetsgraden som skickar ett meddelande via de kanaler som är inställda under [Aviseringar](notifications.md). Upprepade fel i säkerhetskopior och dumpar och misslyckade schemalagda återställningskontroller skickar redan sitt eget meddelande och skickas inte två gånger. |
| **Behåll gamla säkerhetskopior när en källa krymper kraftigt eller skrivs om** | På | Så länge en post har ett öppet fynd för en nästan tom källa, en kraftig krympning eller det mesta av sin data lagrad på nytt lämnar gallring och prune postens gamla säkerhetskopior ifred. Kvittera fyndet eller markera det som väntat för att släppa dem. |

Varje post kan använda sin egen känslighet och sin egen aviseringsnivå. Ställ in dem på postens flik **Verifiering**, eller på sidan **Avvikelser** under **Övervakning**. För en ZFS-post gäller de för varje datauppsättning i dess träd.
