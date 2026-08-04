# Resolução de problemas

Um FAQ curto. Para a tabela completa de resolução de problemas do lado do host do backup de VM por SSH (permissão negada, verificação da chave do host, variáveis de template em falta e mais), consulte o [guia de backup de VM por SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) no GitHub.

## Alguma coisa não está corretamente ligada

Abra `/spike` na interface web. A verificação de integração com o host sonda cada montagem e CLI (socket Docker, libvirt, restic, qemu-img, rclone) e reporta quaisquer peças em falta. Comece por aqui antes de assumir um bug: uma montagem em falta ou um host inacessível aparece de imediato.

## Não consigo alcançar a interface web

O BombVault serve HTTPS logo de início na porta `3443` (certificado autoassinado), por isso abra `https://<your-unraid-ip>:3443`. Aceite o aviso do certificado autoassinado, ou coloque o BombVault por trás de um proxy reverso com o seu próprio certificado. Se correr com `HTTP_ONLY=true`, serve HTTP simples na porta `3000` em vez disso (destinado a uso por trás de um proxy que termina o TLS).

## Perdi a minha APP_KEY

A `APP_KEY` deriva a palavra-passe do repositório restic. Sem ela (e sem o kit de recuperação da chave de encriptação), os backups encriptados não podem ser recuperados. É por isso que o Painel insiste para que transfira o kit de recuperação. Consulte [Externo e recuperação](offsite-recovery.md). Gere uma chave com `openssl rand -hex 32` e guarde-a fora do servidor antes de depender de qualquer backup.

## O backup de VM não se liga

O backup de VM comunica com o libvirt por SSH, nunca por uma montagem.

- Confirme que o SSH está ativado no host e que a chave pública do BombVault está autorizada em `/root/.ssh/authorized_keys` (Definições, Sistema, Backup de VM por SSH mostra a chave e um botão **Testar ligação**).
- Numa rede `br0.x` personalizada, defina `LIBVIRT_HOST` para o IP LAN do seu Unraid (o container não consegue alcançar o host via `host.docker.internal` aí). Ative **Definições, Docker, Host access to custom networks**.
- Se alterou a porta SSH do Unraid, defina `LIBVIRT_SSH_PORT` para corresponder.
- O diagnóstico completo passo a passo (teste de alcance, encaminhamento de VLAN, `Permission denied (publickey)`, `Host key verification failed`) está no [guia de backup de VM por SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Um instantâneo a quente de VM não correu

Os instantâneos a quente precisam do agente convidado qemu instalado na VM e do disco em `/mnt/cache` (ou `/mnt/diskX`), não em `/mnt/user`. Numa VM desligada, o a quente recorre automaticamente ao ordenado. Um backup ordenado desliga a VM, faz backup dos discos, e depois reinicia-a, por isso é sempre consistente.

## Um backup falhou com "repository is already locked"

Isto é geralmente um bloqueio restic órfão deixado para trás quando o container foi atualizado ou reiniciado a meio de uma operação. O BombVault deteta um bloqueio comprovadamente órfão, força a sua limpeza e reexperimenta uma vez, automaticamente. Se persistir, use **Definições, Integridade e manutenção, Desbloquear** para o domínio afetado para limpar um bloqueio preso à mão. Um problema genuíno continua a vir ao de cima em vez de ser escondido.

## A minha cópia externa não aconteceu após um backup

A replicação externa é de melhor esforço por conceção, por isso um percalço externo nunca faz o backup local falhar. Verifique o agendamento externo para esse domínio (Definições, Agendamentos): um agendamento em branco replica após cada backup local, enquanto uma cadência envia com menos frequência. Use **Replicar agora** no separador Externo para uma execução a pedido, e observe o indicador de replicação no Painel.

## Um restauro abortou antes de começar

Antes de qualquer coisa ser parada ou removida, o restauro corre uma verificação de conflitos pré-voo: verifica que o IP estático do container e as portas do host publicadas estão livres. Se outro container já detém uma, aborta com uma mensagem clara e acionável em vez de deixar um restauro a meio. Liberte a porta ou o IP em conflito, e depois reexperimente.

## Uma exportação simples falhou em vez de escrever um ficheiro

Se a encriptação age estiver ligada (Definições) mas não estiver definido nenhum destinatário válido, uma exportação falha com um erro claro em vez de escrever texto simples. Adicione um destinatário válido (uma chave pública age ou uma chave pública SSH), ou desligue a encriptação se pretender que a exportação seja texto simples. Consulte [Funcionalidades](features.md).

## O container continua a reiniciar ou parece não-saudável

O BombVault reporta saudável/não-saudável a partir do seu próprio `/api/health`. Uma ferramenta de auto-recuperação (como o Autoheal) pode reiniciá-lo automaticamente se o motor alguma vez encravar. Verifique o registo do container e o relatório `/spike` para a causa subjacente.

## Ainda preso?

- Leia as páginas completas de [Configuração](configuration.md) e [Externo e recuperação](offsite-recovery.md).
- Pergunte na [thread de suporte do Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Abra uma [issue no GitHub](https://github.com/junkerderprovinz/bombvault/issues).
