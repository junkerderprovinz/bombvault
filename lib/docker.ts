import Docker from "dockerode";
import type { ContainerCreateOptions, ContainerInfo, ContainerInspectInfo } from "dockerode";

// ---------------------------------------------------------------------------
// Narrow types
// ---------------------------------------------------------------------------

/** The subset of ContainerInspectInfo that the docker adapter uses. */
export interface ContainerInspect {
  Id: string;
  Name: string;
  Image: string;
  Config: {
    Image: string;
    Env: string[] | null;
    Cmd: string[] | null;
  };
  HostConfig: {
    Binds: string[] | null;
    PortBindings: Record<string, Array<{ HostIp: string; HostPort: string }>> | null;
    RestartPolicy: { Name: string; MaximumRetryCount?: number } | null;
  };
  Mounts: Array<{
    Type: string;
    Source: string;
    Destination: string;
  }>;
}

/** Options we build for createContainer — only what we populate (YAGNI). */
export interface CreateOptions {
  name: string;
  Image: string;
  Env: string[];
  Cmd: string[];
  HostConfig: {
    Binds: string[];
    PortBindings: Record<string, Array<{ HostIp: string; HostPort: string }>>;
    RestartPolicy: { Name: string; MaximumRetryCount?: number };
  };
}

// ---------------------------------------------------------------------------
// DockerLike — narrow interface used by all helpers (DI seam for tests)
// ---------------------------------------------------------------------------

export interface ContainerHandle {
  id: string;
  inspect(): Promise<ContainerInspectInfo>;
  start(options?: object): Promise<unknown>;
  stop(options?: { t?: number }): Promise<unknown>;
  remove(options?: object): Promise<unknown>;
}

export interface DockerLike {
  listContainers(options?: object): Promise<ContainerInfo[]>;
  getContainer(id: string): ContainerHandle;
  createContainer(options: ContainerCreateOptions): Promise<ContainerHandle>;
  pull(repoTag: string, options?: object): Promise<NodeJS.ReadableStream>;
  modem: {
    followProgress(
      stream: NodeJS.ReadableStream,
      onFinished: (err: Error | null, output: unknown[]) => void,
      onProgress?: (event: unknown) => void,
    ): void;
  };
}

// ---------------------------------------------------------------------------
// Factory — wraps real dockerode with the DockerLike interface
// ---------------------------------------------------------------------------

/**
 * Create a real docker client wrapping dockerode.
 * `pull` drains the progress stream to completion before resolving so the
 * image is guaranteed to be available when the promise settles.
 */
export function createDockerClient(socketPath = "/var/run/docker.sock"): DockerLike {
  const docker = new Docker({ socketPath });
  return {
    listContainers: (options) => docker.listContainers(options),
    getContainer: (id) => docker.getContainer(id) as ContainerHandle,
    createContainer: (options) =>
      docker.createContainer(options) as Promise<ContainerHandle>,
    pull: (repoTag, options = {}) =>
      new Promise<NodeJS.ReadableStream>((resolve, reject) => {
        docker.pull(
          repoTag,
          options,
          (err: Error | null, stream: NodeJS.ReadableStream | undefined) => {
            if (err || !stream) { reject(err ?? new Error("pull returned no stream")); return; }
            (docker.modem as { followProgress: DockerLike["modem"]["followProgress"] }).followProgress(
              stream,
              (ferr) => { if (ferr) reject(ferr); else resolve(stream); },
            );
          },
        );
      }),
    modem: docker.modem as DockerLike["modem"],
  };
}

// ---------------------------------------------------------------------------
// Pure helper — build createContainer options from inspect data
// ---------------------------------------------------------------------------

/**
 * Build a minimal ContainerCreateOptions from a container's inspect data.
 * Strips the leading `/` from the container name (dockerode convention).
 * YAGNI: only maps Image, name, Env, Cmd, Binds, PortBindings, RestartPolicy.
 */
export function buildCreateOptions(inspect: ContainerInspect): CreateOptions {
  const name = inspect.Name.startsWith("/") ? inspect.Name.slice(1) : inspect.Name;
  return {
    name,
    Image: inspect.Config.Image,
    Env: inspect.Config.Env ?? [],
    Cmd: inspect.Config.Cmd ?? [],
    HostConfig: {
      Binds: inspect.HostConfig.Binds ?? [],
      PortBindings: inspect.HostConfig.PortBindings ?? {},
      RestartPolicy: inspect.HostConfig.RestartPolicy ?? { Name: "no" },
    },
  };
}

// ---------------------------------------------------------------------------
// Adapter functions — all accept DockerLike for DI
// ---------------------------------------------------------------------------

/** List all containers (including stopped). */
export async function listContainers(docker: DockerLike): Promise<ContainerInfo[]> {
  return docker.listContainers({ all: true });
}

/** Inspect a single container by ID or name. */
export async function inspectContainer(
  docker: DockerLike,
  id: string,
): Promise<ContainerInspectInfo> {
  return docker.getContainer(id).inspect();
}

/** Stop a container with a configurable timeout (seconds). */
export async function stopContainer(
  docker: DockerLike,
  id: string,
  timeoutSec: number,
): Promise<void> {
  await docker.getContainer(id).stop({ t: timeoutSec });
}

/** Start a container by ID or name. */
export async function startContainer(docker: DockerLike, id: string): Promise<void> {
  await docker.getContainer(id).start();
}

/** Remove a container by ID or name. */
export async function removeContainer(docker: DockerLike, id: string): Promise<void> {
  await docker.getContainer(id).remove();
}

/** Pull an image, draining the progress stream to completion. */
export async function pullImage(docker: DockerLike, repoTag: string): Promise<void> {
  const stream = await docker.pull(repoTag);
  await new Promise<void>((resolve, reject) => {
    docker.modem.followProgress(stream, (err) => {
      if (err) reject(err);
      else resolve();
    });
  });
}

/**
 * Pull the image, create a container from the definition, and start it.
 * Order: pull → createContainer → start.
 */
export async function createAndStartFromDefinition(
  docker: DockerLike,
  inspect: ContainerInspect,
): Promise<ContainerHandle> {
  await pullImage(docker, inspect.Config.Image);
  const options = buildCreateOptions(inspect);
  const container = await docker.createContainer(options as ContainerCreateOptions);
  await container.start();
  return container;
}
