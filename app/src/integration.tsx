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
} from '@pinpt/agent.websdk';
import styles from './styles.module.less';

type Maybe<T> = T | undefined | null;

// interface orgResponse {
// 	avatarUrl: string;
// 	id: string;
// 	name: string;
// 	scopes: string[];
// 	url: string;
// }

// interface projectsResponse {
// 	total: number;
// 	self: string;
// }

// // FIXME
// const AccountListBasic = () => {
// 	return (
// 		<p>basic auth</p>
// 	);
// };

// const fetchOrgsOAuth2 = async (config: Config): Promise<orgResponse[]> => {
// 	let resp = await Http.get('https://api.atlassian.com/oauth/token/accessible-resources', {
// 		'Authorization': 'Bearer ' + config.oauth2_auth!.access_token,
// 		'Content-Type': 'application/json'
// 	});
// 	if (resp[1] !== 200) {
// 		console.error('error fetching orgs', 'response code', resp[1]);
// 		return [];
// 	}
// 	return resp[0] as orgResponse[];

// }

// const fetchProjectParams: string[] = [
// 	'typeKey=software',
// 	'status=live',
// 	'maxResults=100'
// ];

// const fetchProjectCountOAuth2 = async (config: Config, accountId: string): Promise<number> => {
// 	const resp = await Http.get('https://api.atlassian.com/ex/jira/' + accountId + '/rest/api/3/project/search?' + fetchProjectParams.join('&'), {
// 		'Authorization': 'Bearer ' + config.oauth2_auth!.access_token
// 	});
// 	if (resp[1] !== 200) {
// 		if (resp[0].errorMessages?.length) {
// 			throw new Error(resp[0].errorMessages[0]);
// 		}
// 		throw new Error('Error returned fetching projects');
// 	}
// 	const projects = resp[0] as projectsResponse
// 	return projects.total;
// }

// const fetchProjectCountBasicAuth = async (auth: IAuth): Promise<number> => {
// 	const basic = auth as IAppBasicAuth
// 	const resp = await Http.get(basic.url! + '/rest/api/2/project/search?' + fetchProjectParams.join('&'), {
// 		'Authorization': 'Basic ' + btoa(basic.username + ":" + basic.password),
// 	});
// 	if (resp[1] !== 200) {
// 		if (resp[0].errorMessages?.length) {
// 			throw new Error(resp[0].errorMessages[0]);
// 		}
// 		throw new Error('Error returned fetching projects');
// 	}
// 	const projects = resp[0] as projectsResponse
// 	return projects.total;
// }

// const AccountListOAuth2 = () => {
// 	const [loading, setLoading] = useState(true);
// 	const [error, setError] = useState<string>();
// 	const { config, setConfig, installed, setInstallEnabled } = useIntegration();
// 	const [accounts, setAccounts] = useState<Account[]>([]);

// 	useEffect(() => {
// 		const fetch = async () => {
// 			const accts: Account[] = [];
// 			const orgs = await fetchOrgsOAuth2(config);
// 			if (orgs.length === 0) {
// 				setError('No projects found for this organization');
// 				return;
// 			}
// 			config.accounts = {};
// 			await Promise.all(orgs.map(async (current: any) => {
// 				const count = await fetchProjectCountOAuth2(config, current.id);
// 				if (count === 0) {
// 					return;
// 				}
// 				const account: Account = {
// 					id: current.id,
// 					name: current.name,
// 					description: '',
// 					avatarUrl: current.avatarUrl,
// 					totalCount: count,
// 					type: 'ORG',
// 					public: false,
// 				}
// 				accts.push(account);
// 				config.accounts![account.id] = account;
// 			}));
// 			setInstallEnabled(installed ? true : accts.length > 0);
// 			setAccounts(accts);
// 			setConfig(config);
// 			setLoading(false);
// 		}
// 		fetch().catch(err => {
// 			setLoading(false);
// 			setInstallEnabled(false);
// 			setError(err.message);
// 		});
// 	}, []);

// 	if (loading) {
// 		return <Loader>Fetching details</Loader>;
// 	}

// 	return (
// 		<>
// 			{error && <Banner error>{error}</Banner> }
// 			<AccountsTable
// 				description='For the selected accounts, all projects, issues and other data will automatically be made available in Pinpoint once installed.'
// 				accounts={accounts}
// 				entity='project'
// 				config={config}
// 			/>
// 		</>
// 	);
// };

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
				<Icon icon={['fas', 'lock-open']} className={styles.Icon} />
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
				setValidate(config, (err: Maybe<Error>, res: Maybe<any>) => {
					if (err) {
						setButtonText('Validate');
						setUpdatedState(selfManagedFormState.EnteringUrl);
						callback(err);
					} else {
						setButtonText('Begin Setup');
						setUpdatedState(selfManagedFormState.Validated);
					}
				});
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
				u.pathname = '/plugins/servlet/applinks/listApplicationLinks';
				setOAuth1Connect(u.toString(), (err: Maybe<Error>) => {
					setConnected(true);
				});
				const width = window.innerWidth < 1000 ? window.innerWidth : 1000;
				const height = window.innerHeight < 600 ? window.innerHeight : 600;
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

const urlStorageKey = 'installer.jira.url';

enum State {
	Location,
	Setup,
	AgentSelector,
	Link,
	Validate,
	Projects,
}

const Integration = () => {
	const { loading, currentURL, config, isFromRedirect, isFromReAuth, setValidate, setConfig, session, setInstallEnabled } = useIntegration();
	const [type, setType] = useState<IntegrationType | undefined>(config.integration_type);
	const [state, setState] = useState<State>(State.Location);
	const [error, setError] = useState<Error | undefined>();
	const [url, setURL] = useState('');
	const [accounts, setAccounts] = useState<Account[]>([]);

	useEffect(() => {
		if (isFromReAuth) {
			setState(State.AgentSelector);
		}
	}, [isFromReAuth]);

	useEffect(() => {
		if (isFromRedirect && currentURL) {
			const search = currentURL.split('?');
			const tok = search[1].split('&');
			tok.forEach(token => {
				const t = token.split('=');
				const k = t[0];
				const v = t[1];
				if (k === 'result') {
					const result = JSON.parse(atob(decodeURIComponent(v)));
					console.log(result);
					const { success, consumer_key, oauth_token, oauth_token_secret } = result;
					if (success) {
						const url = window.sessionStorage.getItem(urlStorageKey);
						config.oauth1_auth = {
							date_ts: Date.now(),
							url,
							consumer_key,
							oauth_token,
							oauth_token_secret,
						}
						setConfig(config);
						setState(State.Validate);
						window.sessionStorage.removeItem(urlStorageKey);
					} else {
						// FIXME:
					}
				}
			});
		}
	}, [isFromRedirect, currentURL, config, setConfig]);

	const selfManagedCallback = useCallback((err: Error | undefined, theurl?: string) => {
		setError(err);
		if (theurl) {
			const u = new URL(theurl);
			u.pathname = '';
			let url = u.toString();
			if (/\/$/.test(url)) {
				url = url.substring(0, url.length - 1);
			}
			window.sessionStorage.setItem(urlStorageKey, url);
			setURL(url);
			setState(State.Link);
		}
	}, []);

	useEffect(() => {
		if (accounts?.length) {
			config.accounts = {};
			accounts.forEach((acct: Account) => {
				config.accounts![acct.id] = acct;
			});
			setConfig(config);
			setInstallEnabled(Object.keys(config.accounts).length > 0);
		}
	}, [accounts, config, setConfig, setInstallEnabled]);

	useEffect(() => {
		if (state === State.Validate) {
			const _config = {...config, action: 'FETCH_ACCOUNTS'};
			setValidate(_config, (err: Maybe<Error>, res: Maybe<any>) => {
				if (err) {
					setError(err);
				} else {
					if (res?.simulator) {
						setAccounts([{
							id: '1',
							name: 'pinpt-hq',
							description: '',
							avatarUrl: '',
							totalCount: 24,
							type: 'ORG',
							public: false,
						}]);
					} else {
						// FIXME once robin has his fix
						setAccounts([{
							id: '1',
							name: 'pinpt-hq',
							description: '',
							avatarUrl: '',
							totalCount: 24,
							type: 'ORG',
							public: false,
						}]);
					}
					setState(State.Projects);
				}
			});
		}
	}, [state, setState, setAccounts, setValidate, config]);

	if (loading) {
		return <Loader screen />;
	}

	if (error) {
		return <ErrorMessage message={error.message} error={error} />;
	}

	let content;

	switch (state) {
		case State.Location: {
			content = <LocationSelector setType={(val: 'cloud' | 'selfmanaged') => {
				if (val === 'cloud') {
					setType(IntegrationType.CLOUD);
					setState(State.Setup);
				} else {
					setState(State.AgentSelector);
				}
			}} />;
			break;
		}
		case State.AgentSelector: {
			content = <AgentSelector setType={(type: IntegrationType) => {
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
					accounts={accounts}
					entity='project'
					config={config}
				/>
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