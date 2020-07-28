import React, { useEffect, useState, useRef } from 'react';
import { Icon, Loader, } from '@pinpt/uic.next';
import {
	useIntegration,
	Account,
	AccountsTable,
	IntegrationType,
	OAuthConnect,
	IAuth,
	IAppBasicAuth,
	Form,
	FormType,
	Http,
	IOAuth2Auth,
	Config,
} from '@pinpt/agent.websdk';

import styles from './styles.module.less';

interface orgResponse {
	avatarUrl: string;
	id: string;
	name: string;
	scopes: string[];
	url: string;
}
interface projectsResponse {
	total: number;
	self: string;
}

const AccountListBasic = () => {
	return (
		<p>basic auth</p>
	);
}
const fetchOrgsOAuth2 = async (config: Config): Promise<orgResponse[]> => {
	let resp = await Http.get('https://api.atlassian.com/oauth/token/accessible-resources', {
		'Authorization': 'Bearer ' + config.oauth2_auth!.access_token,
		'Content-Type': 'application/json'
	});
	if (resp[1] != 200) {
		console.error('error fetching orgs', 'response code', resp[1]);
		return [];
	}
	return resp[0] as orgResponse[];

}
const fetchProjectCountOAuth2 = async (config: Config, accountId: string): Promise<number> => {
	let params = [
		'expand=' + encodeURIComponent('description,url,issueTypes,projectKeys'),
		'typeKey=software',
		'status=live',
		'maxResults=100'
	];
	let resp = await Http.get('https://api.atlassian.com/ex/jira/' + accountId + '/rest/api/3/project/search?' + params.join('&'), {
		'Authorization': 'Bearer ' + config.oauth2_auth!.access_token
	});
	if (resp[1] != 200) {
		console.error('error fetching projects', 'response code', resp[1]);
		return 0;
	}
	let projects = resp[0] as projectsResponse
	return projects.total;
}

const AccountListOAuth2 = () => {
	const { config, setConfig, installed, setInstallEnabled } = useIntegration();
	const [accounts, setAccounts] = useState<Account[]>([]);

	useEffect(() => {
		const fetch = async () => {
			let accts: Account[] = [];
			let orgs = await fetchOrgsOAuth2(config);
			if (orgs.length === 0) {
				return;
			}
			config.accounts = {};
			for (var i = 0; i < orgs.length; i++) {
				let current = orgs[i];
				let count = await fetchProjectCountOAuth2(config, current.id);
				if (count === 0) {
					return;
				}
				let account: Account = {
					id: current.id,
					name: current.name,
					description: '',
					avatarUrl: current.avatarUrl,
					totalCount: count,
					type: 'ORG',
					public: false
				}
				accts.push(account);
				config.accounts[account.id] = account;
			}
			setInstallEnabled(installed ? true : accts.length > 0);
			setAccounts(accts);
			setConfig(config);
		}
		fetch();
	}, []);

	return (
		<AccountsTable
			description='For the selected accounts, all projects, issues and other data will automatically be made available in Pinpoint once installed.'
			accounts={accounts}
			entity='project'
			config={config}
		/>
	);
};

const LocationSelector = ({ setType }: { setType: (val: IntegrationType) => void }) => {
	return (
		<div className={styles.Location}>
			<div className={styles.Button} onClick={() => setType(IntegrationType.CLOUD)}>
				<Icon icon={['fas', 'cloud']} className={styles.Icon} />
				I'm using the <strong>jira.com</strong> cloud service to manage my data
			</div>

			<div className={styles.Button} onClick={() => setType(IntegrationType.SELFMANAGED)}>
				<Icon icon={['fas', 'server']} className={styles.Icon} />
				I'm using <strong>my own systems</strong> or a <strong>third-party</strong> to manage a Jira service
			</div>
		</div>
	);
};

// TODO:
const SelfManagedForm = () => {
	async function verify(auth: IAuth): Promise<boolean> {
		try {
			return false;
		} catch (ex) {

			return false;
		}
	}
	return <Form type={FormType.BASIC} name='jira' callback={verify} />;
};

const Integration = () => {
	const { loading, currentURL, config, isFromRedirect, isFromReAuth, setConfig } = useIntegration();
	const [type, setType] = useState<IntegrationType | undefined>(config.integration_type);
	const [, setRerender] = useState(0);

	// ============= OAuth 2.0 =============
	useEffect(() => {
		if (!loading && isFromRedirect && currentURL) {
			const search = currentURL.split('?');
			const tok = search[1].split('&');
			tok.forEach(async token => {
				const t = token.split('=');
				const k = t[0];
				const v = t[1];
				if (k === 'profile') {
					const profile = JSON.parse(atob(decodeURIComponent(v)));
					config.oauth2_auth = {
						url: profile.Integration.url,
						access_token: profile.Integration.auth.accessToken,
						refresh_token: profile.Integration.auth.refreshToken,
						scopes: profile.Integration.auth.scopes,
					};
					setConfig(config);
					setRerender(Date.now());
				}
			});
		}

	}, [loading, isFromRedirect, currentURL]);

	useEffect(() => {
		if (type) {
			config.integration_type = type;
			setConfig(config);
			setRerender(Date.now());
		}
	}, [type])

	if (loading) {
		return <Loader screen />;
	}

	let content;

	if (isFromReAuth) {
		if (config.integration_type === IntegrationType.CLOUD) {
			content = <OAuthConnect name='Jira' reauth />;
		} else {
			content = <SelfManagedForm />;
		}
	} else {
		if (!config.integration_type) {
			content = <LocationSelector setType={setType} />;
		} else if (config.integration_type === IntegrationType.CLOUD && !config.oauth2_auth) {
			content = <OAuthConnect name='Jira' />;
		} else if (config.integration_type === IntegrationType.SELFMANAGED && !config.basic_auth && !config.apikey_auth) {
			// content = <SelfManagedForm />;
		} else {
			if (config.oauth2_auth) {
				content = <AccountListOAuth2 />;
			} else {
				content = <AccountListBasic />;
			}
		}
	}

	return (
		<div className={styles.Wrapper}>
			{content}
		</div>
	);
};


export default Integration;