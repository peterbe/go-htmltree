angular.module('htmltree', [
    'ngRoute',
    'classy'
])

.config(['$locationProvider', function ($locationProvider) {
    $locationProvider.html5Mode(true);
}])

.classy.controller({
    name: 'AppController',
    inject: ['$scope', '$http', '$location', '$interval'],
    init: function() {
        //
        // makeTree('flare.json', '#tree');
        // this.$.url='http://www.peterbe.com';
        this.$.url = '';
        this.$.drawn = false;
        this.$.max_depth = 5;
        this.$.loading = false;
        this.$.page_width = 960;
        this.$.server_error = false;
        this.$.stats = {};
        if (this.$location.search().url) {
            this.$.url = this.$location.search().url;
            this._drawTree(this.$location.search().url);
        }

        this.$.recent = [];
        this.$http.get('/tree')
        .success(function(r) {
            if (r.recent) {
                this.$.recent = r.recent;
            }
        }.bind(this));
        // this.$.jobs_in_queue = 0;
        // this.$interval(function() {
        //     this.$http.get('/tree')
        //     .success(function(r) {
        //         this.$.jobs_in_queue = r.jobs;
        //     }.bind(this));
        // }.bind(this), 1000);
    },

    reset: function() {
        this.$.url = '';
        this.$.drawn = false;
        this.$.server_error = false;
        this.$.bad_request_error = false;
        this.$.page_width = 960;
        this.$.stats = {};
        d3.select('#tree svg').remove();
    },

    submitForm: function() {
        if (this.$.url.trim()) {
            this.$location.search('url', this.$.url.trim());
            this._drawTree(this.$.url.trim());
        } else {
            d3.select('#tree svg').remove();
        }
    },

    toggleAdvancedStats: function() {
      this.$.stats._show_advanced = !this.$.stats._show_advanced;
    },

    _drawTree: function(url) {
        this.$.loading = true;
        this.$.server_error = false;
        this.$.page_width = window.innerWidth - 40;
        d3.select('#tree svg').remove();
        this.$http.post(
            '/tree',
            {url: url, max_depth: this.$.max_depth, treemap: true}
        )
        .success(function(response) {
            makeTree(response.nodes, '#tree', this.$.page_width);
            var totalTime = response.performance.download +
              response.performance.parse +
              response.performance.process;
            this.$.stats = {
                size: response.nodes._size, // root node's size
                took: totalTime / 1000, // displayed in seconds
                took_download: response.performance.download,
                took_parse: response.performance.parse,
                took_process: response.performance.process,
            };
        }.bind(this))
        .error(function(data, status, headers) {
            if (status === 500) {
                this.$.server_error = true;
            } else if (status === 400) {
                this.$.bad_request_error = true;
            }
            console.error(data);
            console.error('Status', status);
        }.bind(this))
        .finally(function() {
            this.$.drawn = true;
            this.$.loading = false;
        }.bind(this));
    },

    sampleSubmission: function(url) {
        this.$.url = url;
        this._drawTree(this.$.url.trim());
    }

})

;
