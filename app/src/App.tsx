import React from 'react';
import { SimulatorInstaller, Integration } from '@pinpt/agent.websdk';
import IntegrationUI from './integration';

function App() {
	// check to see if we are running local and need to run in simulation mode
	if (window === window.parent && window.location.href.indexOf('localhost') > 0) {
		const integration: Integration = {
			name: 'Jira',
			description: 'The official Atlassian Jira integration for Pinpoint',
			tags: ['Issue Management'],
			installed: false,
			refType: 'jira',
			icon: 'https://img.icons8.com/color/144/000000/jira.png',
			publisher: {
				name: 'Pinpoint',
				avatar: 'https://avatars0.githubusercontent.com/u/24400526?s=200&v=4',
				url: 'https://pinpoint.com'
			},
			uiURL: document.location.href,
		};
		return <SimulatorInstaller integration={integration} />;
	}
	return (
		<div className="App">
			<IntegrationUI />
		</div>
	);
}

export default App;
