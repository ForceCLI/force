!function ($) {

	$(document).ready(function() {

		function activateDownload(os) {
			$('.download-buttons .button').addClass('hide');
			$('#button-download-'+os).removeClass('hide');

			$('.os-selector-item').removeClass('active');
			$('.os-selector-item-'+os).addClass('active')
		}

		// Determine User's Operating System
		var userOS;
		if (navigator.appVersion.indexOf("Win")!=-1) userOS="windows";
		if (navigator.appVersion.indexOf("Mac")!=-1) userOS="osx";
		if (navigator.appVersion.indexOf("X11")!=-1) userOS="linux";
		if (navigator.appVersion.indexOf("Linux")!=-1) userOS="linux";
		activateDownload(userOS);

		$('.os-selector-item').on('click', function(event) {
			var os = $(this).data('os');
			activateDownload(os);
		})

	});

	$(window).on("load", function() {

		init();

	});

	/* ========================================================================= */
	/*                                                                           */
	/* Initialize Application                                                    */
	/*                                                                           */
	/* ========================================================================= */

	function init() {

		/* --------------------------------------------------
		Initialize Namespace
		-------------------------------------------------- */

		$.app = {};

		// Define variables

		$.app.stage = $("#stage");

		/* --------------------------------------------------
		Initialize Window Resize Methods
		-------------------------------------------------- */

		$(window).resize(function () {

			window_resize();

		});

		window_resize();

		/* --------------------------------------------------
		Initialize Window Scroll Methods
		-------------------------------------------------- */

		$(window).scroll(function () {

			window_scroll();

		});

		window_scroll();

		/* --------------------------------------------------
		Retina Replace
		-------------------------------------------------- */

		$('.retina-replace').retina('@2x');

		/* --------------------------------------------------
		Initialize Google Code Pretty Print
		-------------------------------------------------- */

		window.prettyPrint && prettyPrint();

		/* --------------------------------------------------
		Simulate Placeholders for older browsers
		-------------------------------------------------- */

		simulate_placeholders()

		/* --------------------------------------------------
		Initialize Pages
		-------------------------------------------------- */

		init_pages();

	}

	/* ========================================================================= */
	/*                                                                           */
	/* Initialize Current Page                                                   */
	/*                                                                           */
	/* ========================================================================= */

	function init_pages() {

		if($.app.stage.hasClass("home")) {

			init_home();

		}

	}

	/* ========================================================================= */
	/*                                                                           */
	/* Initialize Home Page                                                      */
	/*                                                                           */
	/* ========================================================================= */

	function init_home() {

	}

	/* ========================================================================= */
	/*                                                                           */
	/* Window Resize                                                             */
	/*                                                                           */
	/* ========================================================================= */

	function window_resize() {

	}

	/* ========================================================================= */
	/*                                                                           */
	/* Window Scroll                                                             */
	/*                                                                           */
	/* ========================================================================= */

	function window_scroll() {

	}

	/* ========================================================================= */
	/*                                                                           */
	/* Get Browser Dimensions                                                    */
	/*                                                                           */
	/* ========================================================================= */

	function get_browserDimensions() {

		var dimensions = {

			width: 0,
			height: 0

		};

		if ($(window)) {

			dimensions.width = $(window).width();
			dimensions.height = $(window).height();

		}

		return dimensions;

	}

	/* ========================================================================= */
	/*                                                                           */
	/* Resolve placeholder text in older browsers                                */
	/*                                                                           */
	/* ========================================================================= */

	function simulate_placeholders() {

		var input = document.createElement("input");

		if(('placeholder' in input) == false) {

			$('[placeholder]').focus(function() {

				var i = $(this);

				if(i.val() == i.attr('placeholder')) {

					i.val('').removeClass('placeholder');

					if(i.hasClass('password')) {

						i.removeClass('password');
						this.type='password';

					}

				}

			}).blur(function() {

				var i = $(this);

				if(i.val() == '' || i.val() == i.attr('placeholder')) {

					if(this.type=='password') {

						i.addClass('password');
						this.type='text';

					}

					i.addClass('placeholder').val(i.attr('placeholder'));

				}

			}).blur().parents('form').submit(function() {

				$(this).find('[placeholder]').each(function() {

					var i = $(this);

					if(i.val() == i.attr('placeholder')) {

						i.val('');

					}

				})

			});

		}

	}

}(window.jQuery)