import { Action, ActionPanel, Clipboard, Color, Icon, List, LocalStorage, showHUD, showToast, Toast } from "@raycast/api";
import { useEffect, useState } from "react";
import { Env, open, Org, run, User } from "./cli";

export default function Command() {
  const [envs, setEnvs] = useState<Env[]>();
  const [env, setEnv] = useState<Env>();
  const orgs = useCLI<Org[]>(env && ["orgs", "--env", env.name], `Loading the ${env?.name} organizations failed`);

  useEffect(() => {
    Promise.all([run<Env[]>(["envs"]), LocalStorage.getItem<string>("env")]).then(
      ([envs, last]) => {
        setEnvs(envs);
        setEnv(envs.find((e) => e.name === last) ?? envs[0]);
      },
      (error) => showFailure("Loading the environments failed", error),
    );
  }, []);

  function pickEnv(name: string) {
    setEnv(envs?.find((e) => e.name === name));
    LocalStorage.setItem("env", name);
  }

  return (
    <List
      isLoading={!envs || orgs.isLoading}
      searchBarPlaceholder="Search organizations"
      searchBarAccessory={
        envs &&
        env && (
          <List.Dropdown tooltip="Environment" value={env.name} onChange={pickEnv}>
            {envs.map((e) => (
              <List.Dropdown.Item key={e.name} title={e.name} value={e.name} icon={envIcon(e, Icon.CircleFilled)} />
            ))}
          </List.Dropdown>
        )
      }
    >
      {env &&
        orgs.data?.map((org) => (
          <List.Item
            key={org.org_id}
            title={org.name}
            icon={envIcon(env, Icon.Building)}
            actions={
              <ActionPanel>
                <Action.Push title="Show Users" icon={Icon.PersonLines} target={<Users env={env} org={org} />} />
                <Action.CopyToClipboard title="Copy Organization ID" content={org.org_id} />
              </ActionPanel>
            }
          />
        ))}
    </List>
  );
}

function Users({ env, org }: { env: Env; org: Org }) {
  const users = useCLI<User[]>(["users", "--env", env.name, org.org_id], `Loading the users of ${org.name} failed`);
  const [showDisabled, setShowDisabled] = useState(false);
  const shown = users.data?.filter((u) => showDisabled || (u.enabled && !u.locked));
  const hidden = (users.data?.length ?? 0) - (shown?.length ?? 0);

  const toggle = (
    <Action
      title={showDisabled ? "Hide Disabled Users" : "Show Disabled Users"}
      icon={showDisabled ? Icon.EyeDisabled : Icon.Eye}
      shortcut={{ modifiers: ["cmd"], key: "d" }}
      onAction={() => setShowDisabled(!showDisabled)}
    />
  );

  return (
    <List
      isLoading={users.isLoading}
      navigationTitle={`${env.name} · ${org.name}`}
      searchBarPlaceholder={`Search the users of ${org.name}`}
    >
      <List.EmptyView
        title={hidden ? `${hidden} disabled or locked users` : "No users"}
        actions={hidden ? <ActionPanel>{toggle}</ActionPanel> : undefined}
      />
      <List.Section title="Users" subtitle={hidden ? `${hidden} hidden` : undefined}>
        {shown?.map((user) => (
          <List.Item
            key={user.user_id}
            title={user.email}
            subtitle={`${user.first_name} ${user.last_name}`.trim()}
            keywords={[user.first_name, user.last_name]}
            accessories={[
              ...(user.enabled ? [] : [{ tag: { value: "disabled", color: Color.SecondaryText } }]),
              ...(user.locked ? [{ tag: { value: "locked", color: Color.Red } }] : []),
              { text: user.role },
            ]}
            actions={
              <ActionPanel>
                <Action title="Impersonate" icon={Icon.Person} onAction={() => impersonate(env, user)} />
                {toggle}
                <Action.CopyToClipboard title="Copy Email" content={user.email} />
                <Action.CopyToClipboard title="Copy User ID" content={user.user_id} />
              </ActionPanel>
            }
          />
        ))}
      </List.Section>
    </List>
  );
}

async function impersonate(env: Env, user: User) {
  try {
    await open(env.name, user.user_id);
    await showHUD(`Opening the impersonation of ${user.email} on ${env.name}`);
  } catch (error) {
    await showFailure("Impersonation failed", error);
  }
}

// useCLI runs impersonate with args, again when they change, and shows its failure in a toast.
function useCLI<T>(args: string[] | undefined, failureTitle: string): { data?: T; isLoading: boolean } {
  const [state, setState] = useState<{ data?: T; isLoading: boolean }>({ isLoading: true });
  const key = args?.join("\0");
  useEffect(() => {
    if (!args) return;
    // A previous environment's answer arriving late must not replace the current one.
    let current = true;
    setState({ isLoading: true });
    run<T>(args).then(
      (data) => current && setState({ data, isLoading: false }),
      (error) => {
        if (!current) return;
        setState({ isLoading: false });
        showFailure(failureTitle, error);
      },
    );
    return () => {
      current = false;
    };
  }, [key]);
  return state;
}

function showFailure(title: string, error: unknown) {
  const message = error instanceof Error ? error.message : String(error);
  return showToast({
    style: Toast.Style.Failure,
    title,
    message,
    primaryAction: { title: "Copy Error", onAction: () => Clipboard.copy(message) },
  });
}

// envIcon tints icon with the environment's color, when it is a hex one rather than an ANSI number.
function envIcon(env: Env, icon: Icon) {
  return { source: icon, tintColor: env.color.startsWith("#") ? env.color : undefined };
}
