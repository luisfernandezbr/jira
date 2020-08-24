import React, { useEffect, useState, useRef, useCallback, useMemo } from 'react';
import { Button, Icon, Loader, Error as ErrorMessage, Theme } from '@pinpt/uic.next';
import {
	useIntegration,
	Account,
	AccountsTable,
	IntegrationType,
	ISession,
	IAuth,
	Form,
	FormType,
	Config,
	URLValidator,
	OAuthConnect,
	OAuthVersion,
	IInstalledLocation,
} from '@pinpt/agent.websdk';
import styles from './styles.module.less';

type Maybe<T> = T | undefined | null;

const LocationSelector = ({ setType }: { setType: (val: 'cloud' | 'selfmanaged') => void }) => {
	return (
		<div className={styles.Location}>
			<div className={styles.Button} onClick={() => setType('cloud')}>
				<Icon icon={['fas', 'cloud']} className={styles.Icon} />
				I'm using the <strong>Atlassian Jira Cloud</strong> service to manage my data
			</div>

			<div className={styles.Button} onClick={() => setType('selfmanaged')}>
				<Icon icon={['fas', 'server']} className={styles.Icon} />
				I'm using <strong>my own systems</strong> or a <strong>third-party</strong> to manage a <strong>Atlassian Jira Server</strong>
			</div>
		</div>
	);
};

const UpgradeRequired = ({ onClick }: { onClick: () => void }) => {
	return (
		<div className={styles.Upgrade}>
			<div className={styles.Content}>
				<h3>Time to upgrade.</h3>
				<p>
					Your current Jira integration is using a legacy configuration and must be re-setup to
					get new features and fixes for Jira. We promise it will be worth it! <span role="img" aria-label="Prayer">🙏</span>
				</p>
				<div>
					To upgrade you have two choices:

					<ol>
						<li>You can delete this Jira integration and then re-add it. <em>Click the Uninstall button above for this option.</em></li>
						<li>You can re-configure this Jira integration. <em>Click the Upgrade button below for this option.</em></li>
					</ol>
				</div>
				<div>
					<Button onClick={onClick} color="Green" weight={500}>Upgrade</Button>
				</div>
				<p className={styles.Help}>
					If you would like someone from our amazing Customer Success team to assist you or if you have any difficulties,
					please don't hesitate to reach out. You can email us at support@pinpoint.com or post a request in the Slack Community.
				</p>
			</div>
		</div>
	);
};

const AgentSelector = ({ setType }: { setType: (val: IntegrationType) => void }) => {
	const { selfManagedAgent, setSelfManagedAgentRequired } = useIntegration();
	const agentEnabled = selfManagedAgent?.enrollment_id;
	const agentRunning = selfManagedAgent?.running;
	const enabled = agentEnabled && agentRunning;
	return (
		<div className={styles.Location}>
			<div className={[styles.Button, enabled ? '' : styles.Disabled].join(' ')} onClick={() => enabled ? setType(IntegrationType.SELFMANAGED) : null}>
				<Icon icon={['fas', 'lock']} className={styles.Icon} />
				I'm using the <strong>Atlassian Jira Server</strong> behind a firewall which is not publically accessible
				<div>
					{agentEnabled && agentRunning ? (
						<>
							<Icon icon="info-circle" color={Theme.Mono300} />
							Your self-managed cloud agent will be used
						</>
					) : !agentEnabled ? (
						<>
							<div><Icon icon="exclamation-circle" color={Theme.Red500} /> You must first setup a self-managed cloud agent</div>
							<Button className={styles.Setup} color="Green" weight={500} onClick={(e: any) => {
								setSelfManagedAgentRequired();
								e.stopPropagation();
							}}>Setup</Button>
						</>
					) : (
						<>
							<div><Icon icon="exclamation-circle" color={Theme.Red500} /> Your agent is not running</div>
							<Button className={styles.Setup} color="Green" weight={500} onClick={(e: any) => {
								setSelfManagedAgentRequired();
								e.stopPropagation();
							}}>Configure</Button>
						</>
					)}
				</div>
			</div>

			<div className={styles.Button} onClick={() => setType(IntegrationType.CLOUD)}>
				<Icon icon={['fas', 'cloud']} className={styles.Icon} />
				I'm using the <strong>Atlassian Jira Server</strong> and it is publically accessible or whitelisted for Pinpoint
				<div>
					<Icon icon="check-circle" color={Theme.Mono300} /> Pinpoint will directly connect to your server
				</div>
			</div>
		</div>
	);
};

const formatUrl = (auth: IAuth | string) => {
	try {
		const u = new URL(auth as string);
		u.pathname = '';
		return u.toString();
	} catch (ex) {
		return auth as string;
	}
};

enum selfManagedFormState {
	EnteringUrl,
	Validating,
	Validated,
	Setup,
}

const SelfManagedForm = ({session, callback, type}: {session: ISession, callback: (err: Error | undefined, url?: string) => void, type: IntegrationType}) => {
	const { setOAuth1Connect, setValidate, id } = useIntegration();
	const [connected, setConnected] = useState(false);
	const [buttonText, setButtonText] = useState('Validate');
	const url = useRef<string>();
	const timer = useRef<any>();
	const windowRef = useRef<any>();
	const state = useRef<selfManagedFormState>(selfManagedFormState.EnteringUrl);
	const [updatedState, setUpdatedState] = useState<selfManagedFormState>();
	const [, setRender] = useState(0);
	const ref = useRef<any>();
	const copy = useCallback(() => {
		if (ref.current) {
			ref.current.select();
			ref.current.setSelectionRange(0, 99999);
			document.execCommand('copy');
		}
	}, [ref]);
	useEffect(() => {
		return () => {
			setOAuth1Connect(''); // unset it
			if (timer.current) {
				clearInterval(timer.current);
				timer.current = null;
			}
			if (windowRef.current) {
				windowRef.current.close();
				windowRef.current = null;
			}
			ref.current = null;
			url.current = '';
		};
	}, [setOAuth1Connect]);
	useEffect(() => {
		if (updatedState) {
			state.current = updatedState;
			setRender(Date.now());
			if (updatedState === selfManagedFormState.Validated) {
				setTimeout(copy, 10);
			}
		}
	}, [updatedState, copy]);
	const verify = useCallback(async(auth: IAuth | string) => {
		switch (state.current) {
			case selfManagedFormState.EnteringUrl: {
				setButtonText('Cancel');
				state.current = selfManagedFormState.Validating;
				const config: Config = {
					integration_type: type,
					url: auth,
					action: 'VALIDATE_URL',
				};
				try {
					await setValidate(config);
					setButtonText('Begin Setup');
					setUpdatedState(selfManagedFormState.Validated);
				} catch (ex) {
					setButtonText('Validate');
					setUpdatedState(selfManagedFormState.EnteringUrl);
					callback(ex);
				}
				break;
			}
			case selfManagedFormState.Validating: {
				// if we get here, we clicked cancel so reset the state
				setButtonText('Validate');
				state.current = selfManagedFormState.EnteringUrl;
				callback(undefined);
				break;
			}
			case selfManagedFormState.Validated: {
				if (windowRef.current) {
					clearInterval(timer.current);
					timer.current = null;
					windowRef.current.close();
					windowRef.current = null;
					callback(undefined, url.current);
					return;
				}
				const u = new URL(auth as string);
				setOAuth1Connect(u.toString(), (err: Maybe<Error>) => {
					setConnected(true);
				});
				const width = window.screen.width < 1000 ? window.screen.width : 1000;
				const height = window.screen.height < 700 ? window.screen.height : 700;
				u.pathname = '/plugins/servlet/applinks/listApplicationLinks';
				windowRef.current = window.open(u.toString(), undefined, `toolbar=no,location=yes,status=no,menubar=no,scrollbars=yes,resizable=yes,width=${width},height=${height}`);
				if (!windowRef.current) {
					callback(new Error(`couldn't open the window to ${auth}`));
					return;
				}
				timer.current = setInterval(() => {
					if (windowRef.current?.closed) {
						clearInterval(timer.current);
						timer.current = null;
						windowRef.current.close();
						windowRef.current = null;
						callback(undefined, auth as string);
					}
				}, 500);
				url.current = auth as string;
				setUpdatedState(selfManagedFormState.Setup);
				setButtonText('Complete Setup');
				break;
			}
			case selfManagedFormState.Setup: {
				if (timer.current) {
					clearInterval(timer.current);
					timer.current = null;
				}
				if (windowRef.current) {
					windowRef.current.close();
					windowRef.current = null;
				}
				setOAuth1Connect('');
				setTimeout(() => callback(undefined, url.current), 1);
				break;
			}
			default: break;
		}
	}, [callback, setOAuth1Connect, setValidate, type]);
	const seed = useMemo(() => String(Date.now()), []);
	let otherbuttons: React.ReactElement | undefined = undefined;
	if (!connected && state.current === selfManagedFormState.Setup) {
		otherbuttons = (
			<Button onClick={() => {
				// reset everything
				if (timer.current) {
					clearInterval(timer.current);
					timer.current = null;
				}
				if (windowRef.current) {
					windowRef.current.close();
					windowRef.current = null;
				}
				setButtonText('Validate');
				setUpdatedState(selfManagedFormState.EnteringUrl);
				setConnected(false);
				url.current = undefined;
				setOAuth1Connect('');
			}}>Cancel</Button>
		);
	}
	return (
		<Form
			type={FormType.URL}
			name='Jira'
			title='Connect Pinpoint to Jira.'
			intro={<>Please provide the URL to your Jira instance and click the button to begin. A new window will open to your Jira instance to authorize Pinpoint to communicate with Jira. Once authorized, come back to this window to complete the connection process. <a rel="noopener noreferrer" target="_blank" href="https://www.notion.so/Pinpoint-Knowledge-Center-c624dd8935454394a3e91dd82bfe341c">Help</a></>}
			button={buttonText}
			callback={verify}
			readonly={state.current === selfManagedFormState.Setup}
			urlFormatter={formatUrl}
			afterword={() => {
				switch (state.current) {
					case selfManagedFormState.EnteringUrl: {
						return <></>;
					}
					case selfManagedFormState.Validating: {
						return (
							<div className={styles.Validating}>
								<Icon icon={['fas', 'spinner']} spin /> Validating
							</div>
						);
					}
					default: break;
				}
				const env = session.env === 'edge' ? 'edge.' : '';
				return (
					<div className={styles.Afterword}>
						<label htmlFor="instructions">Copy this URL and enter it in the "Create new link" field in Jira</label>
						<input ref={ref} type="text" name="instructions" onFocus={copy} readOnly value={`https://auth.api.${env}pinpoint.com/oauth1/jira/${id}/${seed.charAt(seed.length - 1)}`} />
					</div>
				);
			}}
			otherbuttons={otherbuttons}
			enabledValidator={async (url: IAuth | string) => {
				if (url && URLValidator(url as string)) {
					return true;
				}
				return false;
			}}
		/>
	);
};

const upgradeStorageKey = 'installer.jira.in_upgrade';

enum State {
	Location = 1,
	Setup,
	AgentSelector,
	Link,
	Validate,
	Projects,
	UpgradeRequired,
}

const makeAccountsFromConfig = (config: Config) => {
	return Object.keys(config.accounts ?? {}).map((key: string) => config.accounts?.[key]) as Account[];
};

const debugState = false;

const Integration = () => {
	const {
		loading,
		installed,
		currentURL,
		config,
		location,
		isFromRedirect,
		isFromReAuth,
		session,
		upgradeRequired,
		setValidate,
		setConfig, 
		setInstallEnabled,
		setInstallLocation,
		setPrivateKey,
		createPrivateKey,
		getPrivateKey,
		setUpgradeComplete,
	} = useIntegration();
	const [type, setType] = useState<IntegrationType | undefined>(config.integration_type);
	const [state, setState] = useState<State>(State.Location);
	const [error, setError] = useState<Error | undefined>();
	const [url, setURL] = useState('');
	const accounts = useRef<Account[]>([]);
	const currentConfig = useRef<Config>(config);
	const insideRedirect = useRef(false);

	useEffect(() => {
		if ((installed && accounts.current?.length === 0) || config?.accounts) {
			currentConfig.current = config;
			accounts.current = makeAccountsFromConfig(config);
		} else if (currentConfig.current?.accounts) {
			accounts.current = makeAccountsFromConfig(currentConfig.current);
		}
	}, [installed, config, upgradeRequired]);

	const completeUpgrade = useCallback(() => {
		window.sessionStorage.removeItem(upgradeStorageKey);
		setUpgradeComplete();
	}, [setUpgradeComplete]);

	useEffect(() => {
		const inupgrade = window.sessionStorage.getItem(upgradeStorageKey) === 'true';
		if (debugState) {
				console.log('JIRA: state machine', JSON.stringify({
				installed,
				inupgrade,
				location,
				upgradeRequired,
				accounts: config?.accounts,
				isFromReAuth,
				isFromRedirect,
				currentURL,
				insideRedirect: insideRedirect.current,
			}, null, 2));
		}
		if (location) {
			if (location === IInstalledLocation.CLOUD) {
				setInstallLocation(IInstalledLocation.CLOUD);
				setType(IntegrationType.CLOUD);
				setState(State.Setup);
			} else {
				setInstallLocation(IInstalledLocation.SELFMANAGED);
				setState(State.AgentSelector);
			}
		} else if (upgradeRequired && !inupgrade) {
			setState(State.UpgradeRequired);
		} else if (inupgrade && !isFromRedirect) {
			setState(State.AgentSelector);
		} else if (isFromReAuth) {
			setState(State.AgentSelector);
		} else if (installed || config?.accounts) {
			setState(State.Projects);
			if (installed && inupgrade) {
				completeUpgrade();
			}
		} else if (isFromRedirect && currentURL && !insideRedirect.current) {
			const search = currentURL.split('?');
			const tok = search[1].split('&');
			tok.some(token => {
				const t = token.split('=');
				const k = t[0];
				const v = t[1];
				if (k === 'result') {
					const result = JSON.parse(atob(decodeURIComponent(v)));
					const { success, consumer_key, oauth_token, oauth_token_secret, error } = result;
					if (success) {
						const _config = { ...config };
						_config.oauth1_auth = { ...(_config.oauth1_auth || {}), ...{
							date_ts: Date.now(),
							consumer_key,
							oauth_token,
							oauth_token_secret,
						} };
						currentConfig.current = _config;
						insideRedirect.current = true;
						setState(State.Validate);
					} else {
						setError(new Error(error ?? 'Unknown error obtaining OAuth token'));
					}
					return true;
				}
				return false;
			});
		} else if (accounts.current?.length > 0) {
			setState(State.Projects);
		}
	}, [config, location, installed, isFromReAuth, currentURL, isFromRedirect, upgradeRequired, completeUpgrade]);

	const selfManagedCallback = useCallback((err: Error | undefined, theurl?: string) => {
		setError(err);
		if (theurl) {
			const u = new URL(theurl);
			u.pathname = '';
			let url = u.toString();
			if (/\/$/.test(url)) {
				url = url.substring(0, url.length - 1);
			}
			const _config = { ...currentConfig.current } as any;
			_config.oauth1_auth = { ...(_config.oauth1_auth || {}), url };
			setURL(url);
			setState(State.Link);
			setConfig(_config);
			currentConfig.current = _config;
		}
	}, []);

	useEffect(() => {
		if (state === State.Validate && accounts.current?.length === 0) {
			const run = async () => {
				const _config = {...currentConfig.current, action: 'FETCH_ACCOUNTS'};
				try {
					const res = await setValidate(_config);
					const newconfig = { ...currentConfig.current };
					newconfig.accounts = {};
					if (res?.accounts) {
						newconfig.accounts[res.accounts.id] = res.accounts;
					}
					currentConfig.current = newconfig;
					accounts.current = [res.accounts as Account];
					setInstallEnabled(Object.keys(newconfig.accounts).length > 0);
					setState(State.Projects);
					setConfig(currentConfig.current);
				} catch (err) {
					console.error(err);
					setError(err);
				}
			};
			run();
		}
	}, [setInstallEnabled, setValidate, state]);

	if (loading) {
		return <Loader screen />;
	}

	if (error) {
		return <ErrorMessage message={error.message} error={error} />;
	}

	let content;

	switch (state) {
		case State.Location: {
			content = <LocationSelector setType={async (val: 'cloud' | 'selfmanaged') => {
				try {
					const privateKey = await getPrivateKey();
					if (!privateKey) {
						createPrivateKey().then(key => setPrivateKey(key)).catch((err: Error) => {
							setError(err);
						});
					}
					if (val === 'cloud') {
						setType(IntegrationType.CLOUD);
						setState(State.Setup);
					} else {
						setInstallLocation(IInstalledLocation.SELFMANAGED);
						setState(State.AgentSelector);
					}
				} catch (err) {
					setError(err);
				}
			}} />;
			break;
		}
		case State.AgentSelector: {
			content = <AgentSelector setType={async (type: IntegrationType) => {
				setType(type);
				setState(State.Setup);
			}} />;
			break;
		}
		case State.Setup: {
			content = <SelfManagedForm session={session} callback={selfManagedCallback} type={type!} />;
			break;
		}
		case State.Link: {
			content = (
				<OAuthConnect
					name="Jira"
					reauth={false}
					version={OAuthVersion.Version1}
					baseuri={url}
					action="Grant Permission"
					preamble="Your Jira server is now connected to Pinpoint and you need to now authorize Pinpoint to complete setup."
				/>
			);
			break;
		}
		case State.Validate: {
			content = (
				<Loader screen className={styles.Validate}>
					<div>
						<p>
							<Icon icon="check-circle" color={Theme.Green500} /> Connected
						</p>
						<p>Fetching Jira details...</p>
					</div>
				</Loader>
			);
			break;
		}
		case State.Projects: {
			content = (
				<AccountsTable
					description='For the selected accounts, all projects, issues and other data will automatically be made available in Pinpoint once installed.'
					accounts={accounts.current}
					entity='project'
					config={currentConfig.current}
				/>
			);
			break;
		}
		case State.UpgradeRequired: {
			const upgrade = () => {
				window.sessionStorage.setItem(upgradeStorageKey, 'true');
				setState(State.Location);
			};
			content = (
				<UpgradeRequired onClick={upgrade} />
			);
			break;
		}
		default: break;
	}

	return (
		<div className={styles.Wrapper}>
			{content}
		</div>
	);
};


export default Integration;